package conf

import (
	"encoding/json"
	"github.com/BurntSushi/toml"
	"testing"
	"time"
)

/**
 * @BelongProject yunka
 * @BelongPackage conf
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/9/22 11:00 上午
 * @Version V1.0
 */

type database struct {
	Server  string
	Ports   []int
	ConnMax int `toml:"connection_max"`
	Enabled bool
}

type ownerInfo struct {
	Name string
	Org  string `toml:"organization"`
	Bio  string
	Dob  time.Time
}

type clients struct {
	Data  [][]interface{}
	Hosts []string
}

type server struct {
	IP string `toml:"ip"`
	DC string `toml:"dc"`
}

func (o *ownerInfo) String() string {
	bys, _ := json.Marshal(o)
	return string(bys)

}

func TestMap_UnmarshalTOML(t *testing.T) {
	conf := make(Map)

	conf.RegisterType(`title`, ``)
	conf.RegisterType(`owner`, ownerInfo{})
	conf.RegisterType(`database`, database{})
	conf.RegisterType(`servers`, map[string]server{})
	conf.RegisterType(`clients`, clients{})

	if _, err := toml.Decode(`
title = "TOML Example"

[owner]
name = "Tom Preston-Werner"
organization = "GitHub"
bio = "GitHub Cofounder & CEO\nLikes tater tots and beer."
dob = 1979-05-27T07:32:00Z # First class dates? Why not?

[database]
server = "192.168.1.1"
ports = [ 8001, 8001, 8002 ]
connection_max = 5000
enabled = true

[servers]

  # You can indent as you please. Tabs or spaces. TOML don't care.
  [servers.alpha]
  ip = "10.0.0.1"
  dc = "eqdc10"

  [servers.beta]
  ip = "10.0.0.2"
  dc = "eqdc10"

[clients]
data = [ ["gamma", "delta"], [1, 2] ] # just an update to make sure parsers support it

# Line breaks are OK when inside arrays
hosts = [
  "alpha",
  "omega"
]
`, &conf); err != nil {
		t.Fatal(err)
	}
	t.Log(conf[`title`].Value.(string))
	t.Log(conf[`database`].Value.(database))
	t.Log(conf[`owner`].Value.(ownerInfo))
	t.Log(conf[`clients`].Value.(clients))
	t.Log(conf[`clients`].Value.(clients).Hosts)
	t.Log(conf[`clients`].Value.(clients).Data)
	t.Log(conf[`servers`].Value.(map[string]server))
}

type DbCfg struct {
	Schema string `toml:"schema"`
	DbType string `toml:"dbType"`
}

type AliPayApp struct {
	AppId             string
	CallHost          string
	ServiceProviderId string
	PrivateKey        string
	PublicKey         string
}

type WxPayApp struct {
	AppID       string
	MchID       string
	AppSecret   string
	MchSecret   string
	V3Key       string
	CertPemPath string
	CertKeyPath string
}

type SzABBankPay struct {
	CKey       string
	CKey16     string
	SendSite   string
	Method     string
	GatewayUrl string
}

type AppConfig struct {
	Debug               bool
	InitDb              bool
	Es                  bool
	Database            DbCfg `toml:"database"`
	NodeID              string
	QrTmpl              string
	DrCoffeeAuditAliUrl string
	AliPay              AliPayApp
	WxPay               WxPayApp
	SzAB                SzABBankPay
	AliPayCb            string
	PayHost             string
	Ip                  string
	Port                string
	Common              ServerCommon
	TTL                 int `toml:"ttl"`
	Rpc                 Rpc
	AliCard             AliCardCfg `toml:"aliCard"`
	Nsq                 NsqConf
	Influx              InfluxDb
	AesKey              string
	EnterpriseAppConf
	MessageConf
	WebSiteConf
	SpaceConf
	DeviceClusterNode []string
	NotifyClusterNode []string
}

type ServerCommon struct {
	ServerAddress []string
	ServerPrefix  string
	IpPort        string
	TTL           int64
}

type InfluxDb struct {
	Server   string
	Admin    string
	Password string
}

type AliCardCfg struct {
	Host    string
	Path    string
	AppCode string
}

type Rpc struct {
	Ip     string
	Port   string
	NodeID string
	TTL    int `toml:"ttl"`
}

type Cache struct {
	Addr     string
	Password string
}

type EnterpriseAppConf struct {
	Cache  Cache
	EsHost string
	Key    string
}

type WxCfg struct {
	AppId, AppSecret, OauthDomain, RedirectUrl string
}

type MiniPro struct {
	AppId, Jump, Tmpl string
}

type MessageConf struct {
	WxWeb   WxCfg
	MiniPro MiniPro
}

type WebSiteConf struct {
	RowJumpPath []string `toml:"rowJumpPath"`
}

type NsqConf struct {
	Address   string
	Partition int
}

type Oss struct {
	OssKey     string
	OssSecret  string
	ImgDomain  string
	Bucket     string
	UseCname   bool
	Acl        string
	CallSchema string
	CallHost   string
}

type SpaceConf struct {
	Oss Oss
}

type ImConf struct {
	AesKey string
}

type ValveConf struct {
	TTL int64
}

func TestMap_UnmarshalTOMLII(t *testing.T) {
	conf := make(Map)

	conf.RegisterType(`ip`, ``)
	conf.RegisterType(`port`, ``)
	conf.RegisterType(`nodeID`, ``)
	conf.RegisterType(`ttl`, 0)
	conf.RegisterType(`payHost`, ``)
	conf.RegisterType(`deviceClusterNode`, []string{})
	conf.RegisterType(`key`, ``)
	conf.RegisterType(`aesKey`, ``)
	conf.RegisterType(`debug`, false)
	conf.RegisterType(`initDb`, false)
	conf.RegisterType(`qrTmpl`, ``)
	conf.RegisterType(`drCoffeeAuditAliUrl`, ``)
	conf.RegisterType(`database`, DbCfg{})
	conf.RegisterType(`cache`, Cache{})
	conf.RegisterType(`aliPay`, AliPayApp{})
	conf.RegisterType(`WxPay`, WxPayApp{})
	conf.RegisterType(`SzAB`, SzABBankPay{})
	conf.RegisterType(`oss`, Oss{})
	conf.RegisterType(`nsq`, NsqConf{})
	conf.RegisterType(`influx`, InfluxDb{})
	conf.RegisterType(`valve`, ValveConf{})

	if _, err := toml.Decode(`
#服务相关参数配置
ip = ""
port = "localhost:8076"
nodeID = "api_node_01"
ttl=120
payHost="http://.it.com"
deviceClusterNode = ["127.0.0.1:9011"]



[nsq]
address="127.0.0.1:4150"
partition=0

[influx]
server="http://127.0.0.1:8086"
admin="root"
password="test-password"



[valve]
ttl=60

`, &conf); err != nil {
		t.Fatal(err)
	}

	t.Log(conf[`ip`].Value.(string))
	t.Log(conf[`port`].Value.(string))
	t.Log(conf[`nodeID`].Value.(string))
	t.Log(conf[`ttl`].Value.(int))
	t.Log(conf[`payHost`].Value.(string))
	t.Log(conf[`deviceClusterNode`].Value.([]string))
	t.Log(conf[`key`].Value.(string))
	t.Log(conf[`aesKey`].Value.(string))
	t.Log(conf[`debug`].Value.(bool))
	t.Log(conf[`initDb`].Value.(bool))
	t.Log(conf[`qrTmpl`].Value.(string))
	t.Log(conf[`drCoffeeAuditAliUrl`].Value.(string))
	t.Log(conf[`database`].Value.(DbCfg))
	t.Log(conf[`cache`].Value.(Cache))
	t.Log(conf[`aliPay`].Value.(AliPayApp))
	t.Log(conf[`WxPay`].Value.(WxPayApp))
	t.Log(conf[`SzAB`].Value.(SzABBankPay))
	t.Log(conf[`oss`].Value.(Oss))
	t.Log(conf[`nsq`].Value.(NsqConf))
	t.Log(conf[`influx`].Value.(InfluxDb))
	t.Log(conf[`valve`].Value.(ValveConf))
}
