package main

import (
	"fmt"
	"github.com/devfacet/gocmd"
	"github.com/robfig/cron"
	"github.com/zzyandzzy/ptrss/db"
	"github.com/zzyandzzy/ptrss/util"
	"net/http"
	"strconv"
)

type FLAGS struct {
	Help    bool `short:"h" long:"help" description:"show this help message" global:"true"`
	Version bool `short:"v" long:"version" description:"show version num" global:"true"`
	Client  struct {
		Name string `long:"name" required:"true" description:"客服端类型 (tr 或 qb)"`
		Host string `long:"host" required:"true" description:"客服端地址 (http://192.168.0.1)"`
		Port int    `long:"port" required:"true" description:"客服端端口 (9091)"`
		User string `long:"user" required:"true" description:"客服端登录用户 (root)"`
		Pwd  string `long:"pwd" required:"true" description:"客服端登录密码 (root)"`
	} `command:"client" description:"设置客服端参数"`
	Add struct {
		Url     string `long:"url" required:"true" description:"添加RSS地址 (请把地址用\"\"包围)"`
		Client  string `long:"client" required:"true" description:"使用哪种客服端下载 (tr 或 qb)"`
		Path    string `long:"path" description:"设置下载路径 (完整路径: /volume2/download/) [默认: 客服端设置]"`
		Pause   string `long:"pause" description:"是否暂停下载 (true 或 false)[默认: true]"`
		Refresh int    `long:"refresh" description:"设置自动刷新时间 (单位: 秒)[默认: 300]"`
		Filter  struct {
			Name string `long:"name" description:"根据名称过滤rss内容"`
			Size int    `long:"size" description:"根据大小过滤rss内容"`
			Path string `long:"path" description:"设置过滤下载路径 [默认: 客服端设置]"`
		} `command:"filter" description:"添加RSS过滤器"`
	} `command:"add" description:"添加RSS订阅"`
	Get struct {
		Client string `long:"client" description:"根据客服端名字获取已添加的客服端参数 (tr 或 qb)"`
		RSS    string `long:"rss" description:"根据客服端名字获取已添加rss的参数 (tr 或 qb)"`
	} `command:"get" description:"获取客服端参数"`
}

var (
	defaultPause        = true
	defaultRefresh      = 300
	defaultTrClientName = "tr"
	flags               = FLAGS{}
)

func main() {
	db.Init()
	Cmd()
}

func Cmd() {
	gocmd.HandleFlag("Add", func(cmd *gocmd.Cmd, args []string) error {
		pause := defaultPause
		if flags.Add.Pause == "false" || flags.Add.Pause == "FALSE" {
			pause = false
		}
		refresh := defaultRefresh
		if flags.Add.Refresh != 0 {
			refresh = flags.Add.Refresh
		}
		c := cron.New()
		c.AddFunc("*/"+strconv.Itoa(refresh)+" * * * * ?", func() {
			AddRSS(flags.Add.Client, flags.Add.Url, flags.Add.Path, pause, refresh)
		})
		c.Start()
		defer c.Stop()
		select {}
		return nil
	})

	gocmd.HandleFlag("Client", func(cmd *gocmd.Cmd, args []string) error {
		host := flags.Client.Host
		port := flags.Client.Port
		local := host + ":" + strconv.Itoa(port)
		user := flags.Client.User
		pwd := flags.Client.Pwd
		fmt.Printf("正在测试 %s 的可连接性\n", local)
		if flags.Client.Name == defaultTrClientName {
			AddTrClient(local, user, pwd)
		}
		return nil
	})

	gocmd.HandleFlag("Get", func(cmd *gocmd.Cmd, args []string) error {
		if flags.Get.Client != "" {
			client := db.QueryClient(flags.Get.Client)
			if client.Id != 0 {
				fmt.Printf("%#v\n", client)
			} else {
				PrintNonClientHelp()
			}
		}
		if flags.Get.RSS != "" {
			for _, rss := range db.QueryRSS(flags.Get.RSS) {
				fmt.Printf("%#v\n", rss)
			}
		} else {
			PrintNonRSSHelp()
		}
		return nil
	})

	// Init the app
	gocmd.New(gocmd.Options{
		Name:        "PT RSS",
		Version:     "0.0.1",
		Description: "一个简单高效的PT RSS自动化工具",
		Flags:       &flags,
		ConfigType:  gocmd.ConfigTypeAuto,
	})
}

func AddRSS(clientName string, url string, path string, pause bool, refresh int) {
	clientDB := db.QueryClient(clientName)
	if clientDB.Id != 0 {
		client := util.Client{Local: clientDB.Local, User: clientDB.User, Pwd: clientDB.Pwd}
		util.GetBody(url, func(channel util.RSSChannel) {
			var input string
			// 先判断rss有没有添加进数据库
			// 没有说明是第一次添加
			if !db.ExistRSS(url) {
				db.InsertRSS(url, clientDB.Id, path, pause, refresh)
				fmt.Printf("获取到 %s 共 %d 条信息\n是否要将全部添加到客服端？(Y/n): ",
					channel.Title, len(channel.Items))
				_, err := fmt.Scanf("%s", &input)
				if err != nil {
					fmt.Println("输入错误，默认不添加到客服端")
				}
			}
			for _, rssItem := range channel.Items {
				// 判断是否是第一次
				if input == "Y" || input == "y" {
					// 下载rss到tr客服端
					addRSSClient(clientName, path, pause, client, rssItem)
				} else if input == "n" || input == "N" {
					// 直接添加到数据库
					db.InsertData(rssItem.GuidValue, rssItem.Title, rssItem.Enclosure.Url, rssItem.Link,
						rssItem.Enclosure.Type, rssItem.Enclosure.Length, rssItem.PubDate)
				}
				// 不是第一次
				// 判断当前数据有没有添加进数据库，检查增量
				// 没有添加的就添加
				if !db.ExistData(rssItem.GuidValue) {
					addRSSClient(clientName, path, pause, client, rssItem)
				}
			}
		})
	} else {
		PrintNonClientHelp()
	}
}

func addRSSClient(clientName string, path string, pause bool, client util.Client, rssItem util.RSSItem) {
	if clientName == defaultTrClientName {
		util.TrAdd(client, util.TransmissionAdd{
			"", rssItem.Enclosure.Url,
			path, pause},
			func(body []byte, res *http.Response, addResult util.TransmissionAddResult, err error) {
				torrent := addResult.Result.Arguments.(map[string]interface{})
				tid := torrent["id"]
				hash := torrent["hashString"]
				name := torrent["name"]
				// 插入数据到数据库
				db.InsertDatas(tid, rssItem.GuidValue, hash, rssItem.Title, name, rssItem.Enclosure.Url,
					rssItem.Link, rssItem.Enclosure.Type, rssItem.Enclosure.Length, rssItem.PubDate)
				if addResult.Flag == "add" {
					fmt.Printf("添加种子 %s 成功, 是否自动下载: %t\n", torrent["name"], !pause)
				} else if addResult.Flag == "duplicate" {
					fmt.Printf("重复的种子 %s, 不会被添加\n", torrent["name"])
				}
			})
	}
}

// 添加tr客户端
func AddTrClient(local string, user string, pwd string) {
	tr := util.Client{Local: local, User: user, Pwd: pwd}
	util.TrConnect(tr, func(body []byte, res *http.Response, result util.TransmissionResult, err error) {
		// 未连接上，可能会有未知错误或者客服端错误的情况
		if err != nil {
			fmt.Printf("添加客服端错误, 错误信息: %s\n", err.Error())
		} else {
			// 地址有效但可能密码错误
			if res.StatusCode == 200 {
				// 连接成功
				if result.Result == "success" {
					// 插入数据
					if db.InsertClient(defaultTrClientName, local, user, pwd) {
						fmt.Println("添加客服端成功")
					} else {
						fmt.Print("插入数据库失败, 可能已经添加\n")
						PrintClientHelp()
					}
				}
			} else {
				fmt.Println("添加客服端失败，可能是账号密码不正确")
			}
		}
	})
}

func PrintClientHelp() {
	fmt.Println("使用命令 './ptrss get --client' 查看已添加的客服端")
}

func PrintNonClientHelp() {
	fmt.Print("还没有添加客服端, ")
	PrintClientHelp()
}

func PrintRSSHelp() {
	fmt.Println("使用命令 './ptrss get --rss' 查看已订阅的RSS")
}

func PrintNonRSSHelp() {
	fmt.Print("还没有添加RSS, ")
	PrintClientHelp()
}
