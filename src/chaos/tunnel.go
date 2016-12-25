package chaos

/*
sock5_req<->Tunnel<=>fwd<=>server
其中sock5_req<=>server之间并没有建立tcp链接，所以udp丢包了就无法依靠tcp协议自动重发，这个需要chaos自己保证不丢包
除非所有的包全部丢掉了sock5_req才会重新发req，整个链路才会重复，否则只要一个包收发成功sock5_req就无法判断丢包
soc5_req所有的包都发给Tunnel了，这个tcp是非常可靠的，所以他总是认为tcp是完全正常的
同理fwd<=>server也是，server与fwd如果收发全部正常，下面的丢包server是完全无法知道
*/

/*
udp tunnel
send_span 初始500ms，以后通讯成功一次就重新计算这个值，设置成最后一分钟内的平均值
收到数据间隔500ms发送一次，直到收到ack为止
*/
import (
	"crypto/md5"
	"encoding/binary"
	"math/rand"
	"net"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

//ip,port
type Sock5Key struct {
	ip   [4]byte
	port uint16
}

func NewSock5Key(addr *net.TCPAddr) Sock5Key {
	sk := Sock5Key{}
	copy(sk.ip[:], addr.IP.To4())
	sk.port = uint16(addr.Port)
	return sk
}

// func NewSock5Key(addr* net.TCPAddr)Sock5Key {
// 	return Sock5Key{ip:}
// }
//只有这个package使用
type sock5_req_value struct {
	conn        *net.TCPConn
	rx_pack_cnt uint32            //这儿保存接收到的pack_cnt，发送的在tcp相关的结构体里
	raw_data    map[uint32][]byte //cnt,raw_data 保存了还没用到的数据
}

//包装起来给chan使用
type req_kv struct {
	req  *Sock5Key
	conn *net.TCPConn
}

const max_udp_tunnel = 5

//sock5_req<->Tunnel<=>fwd<=>server
//整个流程都是以sock5_req发起的ip,port为标识，所以tunnel两端都用这个ip,port来标记一个链接
//fwd端的tunnel也是用ip,port标识
type Tunnel struct {
	conn         [max_udp_tunnel]*net.UDPConn
	udp2req_chan chan []byte                   //5个udp接收之后通过这个chan收集
	add_req_chan chan *req_kv                  //tcp请求的信息，对应sock5_req或者server
	del_req_chan chan *Sock5Key                //tcp读取错误的时候发送key来删除req_map的某项
	req_map      map[Sock5Key]*sock5_req_value //保存ip,port=>conn,cnt的映射
}

//sock5 req进来后保存起来
func (this *Tunnel) AddReq(conn *net.TCPConn, key *Sock5Key) {
	kv := &req_kv{req: key, conn: conn}
	this.add_req_chan <- kv
}

func (this *Tunnel) DelReq(key *Sock5Key) {
	this.del_req_chan <- key
}

//向tunnel里写数据
func (this *Tunnel) Write(data []byte, key *Sock5Key, cnt uint32) {
	data_len := len(data)
	//raw_data := data
	data = data[:data_len+32]             //扩展32字节
	data_proto := data[:data_len+16]      //数据和协议部分，不包含md5
	proto := data[data_len : data_len+16] //协议部分
	md5_data := data[data_len+16:]        //md5域

	this.write_proto(proto, key.ip[:], key.port, cnt, false)

	//加md5
	m := md5.New()
	md5_r := m.Sum(data_proto)
	copy(md5_data, md5_r)

	//实际发送出去
	for _, c := range this.conn {
		c.Write(data)
	}
}

//tunnel读取与转发给tcp
func (tihs *Tunnel) Handle() {
	for _, c := range tihs.conn {
		go tihs.handle_udp_recv(c) //5个tunnel的接收，然后转给udp2req_chan
	}

	tihs.handle_udp2req() //从udp2req_chan里接收5个tunnel数据，判断、缓存、发送
}

/*
data:通过udp发送出去的数据
addr:sock5请求的ip+port，可以唯一标识数据来源的
cnt:udp包计数器

data后面必须至少保留32字节用来填充协议
*/
//sock5 req收到数据包从这儿发送

//net.IP也可以传进来
func (this *Tunnel) write_proto(proto []byte, ip []byte, port uint16, cnt uint32, close_conn bool) {
	//ip(4),cnt(4),port(2),close(1),rand(5)
	copy(proto[:4], ip)
	proto = proto[4:]

	binary.LittleEndian.PutUint32(proto, cnt)
	proto = proto[4:]

	binary.LittleEndian.PutUint16(proto, uint16(port))
	proto = proto[2:]

	if close_conn {
		proto[0] = 1
	} else {
		proto[0] = 0
	}
	proto = proto[1:]

	//剩下的全部rand
	rand.Read(proto)
}

func (this *Tunnel) handle_udp_recv(c *net.UDPConn) {
	for {
		//修改从pool里获取
		data := make([]byte, 1500)
		n, err := c.Read(data)
		if err != nil {
			//log
		}
		this.udp2req_chan <- data[:n]
	}
}

func (this *Tunnel) extract_proto(data []byte) (*Sock5Key, uint32, bool, []byte) {
	data_len := len(data) - 32
	raw_data := data[:data_len]
	proto := data[data_len:]
	//md5_data := proto[16:]

	key := &Sock5Key{}
	//var ip [4]byte
	//key.ip[:] = proto[:4]
	copy(key.ip[:], proto[:4])
	proto = proto[4:]

	var cnt uint32
	binary.LittleEndian.Uint32(proto)
	proto = proto[4:]

	key.port = binary.LittleEndian.Uint16(proto)
	proto = proto[2:]

	closed := false
	if proto[0] == 1 {
		closed = true
	}
	proto = proto[1:]

	return key, cnt, closed, raw_data
}

//ip,port为key
//tcp_conn,cnt为value，这个是sock5_req的来源和当前的计数

//将udp发来的数据分发到对应的req
func (this *Tunnel) handle_udp2req() {
	for {
		d := <-this.udp2req_chan
		//将所有需要删除和增加的conn都检查一遍
		this.handle_add_del_conn()
		key, cnt, closed, raw_data := this.extract_proto(d)

		v, ok := this.req_map[*key]

		//找不到就不管了，直接丢弃
		if !ok {
			continue
		}

		//给fwd服务器发送ack，这个可能会造成发送多次ack，最多5*5次
		//不能在check_pack_cnt之后发，如果中间丢包，造成后面的包ack也不能发出去，需要标记已经发出的ack
		this.pack_ack(key, v, cnt)

		//远程tcp关闭了，删除入口
		if closed {
			delete(this.req_map, *key)
			continue
		}

		//检查数据包编号是否正常
		ok = this.check_pack_cnt(v, cnt, raw_data)

		//正常了就发送
		if ok {
			if _, err := v.conn.Write(raw_data); err != nil {
				//发送失败删除入口
				delete(this.req_map, *key)
				continue
			}
		}

		for {
			//检查后续所有的包
			d := this.check_cache_pack(v)
			if d == nil {
				break
			}
			//找到了就发送
			if _, err := v.conn.Write(d); err != nil {
				//出错了就删除
				delete(this.req_map, *key)
				break
			}
		}
	}
}

//检查后续的pack_cnt是否在缓存里
func (this *Tunnel) check_cache_pack(v *sock5_req_value) []byte {
	d, ok := v.raw_data[v.rx_pack_cnt]

	if !ok {
		return nil
	}

	//这个cnt已经完成，判断下一个cnt
	v.rx_pack_cnt++

	return d
}

//检查pack_cnt是否正常
func (this *Tunnel) check_pack_cnt(v *sock5_req_value, cnt uint32, raw_data []byte) bool {

	//当前的cnt比已经发送完毕的cnt还小(或者相等），那么数据包就没用了，直接丢掉
	if cnt <= v.rx_pack_cnt {
		return false
	}

	//收到的数据包是上次发送的编号+1，这个是正确的
	if cnt == v.rx_pack_cnt+1 {
		v.rx_pack_cnt++
		return true
	}

	//前面的包还没到，这个包提早了，需要先保存起来
	//先检查其他4个udp是否已经保存这个pack了
	if _, ok := v.raw_data[cnt]; !ok {
		v.raw_data[cnt] = raw_data
	}
	return false
}

//这个不需要发送了，500ms会再发送一次
// func (this *Tunnel) remote_resend_udp(key *Sock5Key, cnt uint32) {
// }

//收到了就朝5个udp都发送，对方只要收到了一个就可以
func (this *Tunnel) pack_ack(k *Sock5Key, v *sock5_req_value, cnt uint32) {
	ack := make([]byte, 36)
	binary.LittleEndian.PutUint32(ack, cnt)
	proto := ack[4:]
	md5_data := proto[16:]
	proto = proto[:16]
	//ack的cnt为0，这个表示当前包不属于数据包，是通讯包
	//cnt从100开始，100以下都表示消息类型
	this.write_proto(proto, k.ip[:], k.port, 0, false)
	m := md5.New()
	copy(md5_data, m.Sum(ack[:20]))

	for _, c := range this.conn {
		c.Write(ack)
	}
}

func (this *Tunnel) handle_add_del_conn() {
	for {
		select {
		case kv := <-this.add_req_chan:
			if _, ok := this.req_map[*kv.req]; !ok {
				this.req_map[*kv.req] = &sock5_req_value{conn: kv.conn, rx_pack_cnt: 100}
			}
		case k := <-this.del_req_chan:
			delete(this.req_map, *k)
		default:
			return
		}
	}

}
