package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"net/http"
)

var port = "80"
var baseUri = "http://localhost"
var minDelay = 0
var maxDelay = 10000

type MenuParam struct {
	BaseUri  string
	MinDelay int
	MaxDelay int
}

func main() {
	// 環境変数からポートを設定
	sPort := os.Getenv("RAGUDELAY_PORT")
	if sPort != "" {
		port = sPort
	}

	// 環境変数からベースURIを設定
	sBaseUri := os.Getenv("RAGUDELAY_BASE_URI")
	if sBaseUri != "" {
		baseUri = sBaseUri
	}

	// 環境変数から遅延設定最小値を設定
	sMinDelay := os.Getenv("RAGUDELAY_MIN")
	if sMinDelay != "" {
		n, err := strconv.Atoi(sMinDelay)
		if err == nil {
			minDelay = n
		}
	}

	// 環境変数から遅延設定最大値を設定
	sMaxDelay := os.Getenv("RAGUDELAY_MAX")
	if sMinDelay != "" {
		n, err := strconv.Atoi(sMaxDelay)
		if err == nil {
			maxDelay = n
		}
	}

	// view
	http.HandleFunc("/menu", serveMenu)

	// api
	http.HandleFunc("/api/set-delay", setDelay)
	http.HandleFunc("/api/get-delay", getDelay)
	http.HandleFunc("/api/check-proxy", checkPloxy)
	http.HandleFunc("/api/disconnect", disconnect)
	http.HandleFunc("/api/connect", connect)

	// サーバーを起動
	fmt.Println("")
	fmt.Println("==============================================")
	fmt.Println("らぐらぐ遅延スタート")
	fmt.Println("")
	fmt.Println("  らぐらぐ遅延スタートへようこそ")
	fmt.Println("")
	fmt.Println("  管理コンソールのURL:")
	fmt.Printf("  %s/menu\n", baseUri)
	fmt.Println("==============================================")
	fmt.Println("")

	http.ListenAndServe(":"+port, nil)
}

// vueのメニューを返すためのハンドラ
func serveMenu(w http.ResponseWriter, r *http.Request) {
	menu := template.Must(template.ParseFiles("views/menu.html"))
	// 遅延の最大、最小を渡す
	err := menu.Execute(w, MenuParam{
		BaseUri:  baseUri,
		MinDelay: minDelay,
		MaxDelay: maxDelay,
	})
	if err != nil {
		log.Fatalln(err)
	}
}

// 遅延設定時間を取得するAPIのハンドラ
func getDelay(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("tc", "qdisc", "show", "dev", "eth0").Output()

	if false && err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "遅延時間の取得に失敗しました")
		return
	}

	sDelay := string(out)
	// sDelay = "qdisc netem 8001: root refcnt 9 limit 1000 delay 10.0s\n"

	// コマンド標準出力から分解
	index := strings.Index(sDelay, "delay ")
	if index == -1 {
		w.WriteHeader(200)
		fmt.Fprintf(w, "0")
		return
	}
	sDelay = sDelay[index+6:]
	index = strings.Index(sDelay, "s")
	if index == -1 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "遅延時間の取得に失敗しました(文字列解析エラー)")
		return
	}
	sDelay = sDelay[:index+1]

	index = strings.Index(sDelay, "ms")

	var num = 0.0
	if index != -1 {
		// ミリ秒表示の場合
		snum := sDelay[:index]
		num, err = strconv.ParseFloat(snum, 64)
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "遅延時間の取得に失敗しました(ミリ秒変換エラー)")
			return
		}
	} else {
		// 秒表示の場合
		index = strings.Index(sDelay, "s")
		snum := sDelay[:index]
		num, err = strconv.ParseFloat(snum, 64)
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "遅延時間の取得に失敗しました(秒変換エラー)")
			return
		}
		num = num * 1000
	}

	// 遅延設定時間を返す
	w.WriteHeader(200)
	fmt.Fprintf(w, fmt.Sprintf("%d", int(num)))
}

// 遅延設定時間を変更するAPIのハンドラ
func setDelay(w http.ResponseWriter, r *http.Request) {
	sDelay := r.URL.Query().Get("delay")
	if sDelay == "" {
		w.WriteHeader(400)
		fmt.Fprintf(w, "設定遅延時間を指定してください")
		return
	}
	nDelay, err := strconv.Atoi(sDelay)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "設定遅延時間は数値を指定してください")
		return
	}
	if minDelay <= nDelay && nDelay <= maxDelay {
		delay := nDelay
		out, err := exec.Command("tc", "qdisc", "change", "dev", "eth0", "root", "netem", "delay", fmt.Sprintf("%dms", delay)).Output()
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "遅延設定コマンドの実行に失敗しました\n"+string(out)+"\ntc qdisc change dev eth0 root netem delay "+fmt.Sprintf("%dms", delay))
			fmt.Printf("遅延設定コマンドの実行に失敗しました\n%s\n", string(out))
			return
		}
		fmt.Printf("遅延を%dミリ秒に指定しました\n"+string(out), delay)
		w.WriteHeader(200)
		fmt.Fprintf(w, fmt.Sprintf("遅延を%dミリ秒に指定しました\n"+string(out), delay))
		return
	} else {
		w.WriteHeader(400)
		fmt.Fprintf(w, fmt.Sprintf("設定遅延時間は%d以上%d以下で設定してください", minDelay, maxDelay))
		return
	}
}

// 切断APIのハンドラ
// 実行するとファイアウォール拒否ルールに3128番ポートを登録(ヨシッ！)
func disconnect(w http.ResponseWriter, r *http.Request) {
	fmt.Println("切断を受信")

	st, err := checkProxyCommand()
	if err != nil {
		fmt.Println("プロキシの状態確認に失敗しました")
		w.WriteHeader(400)
		fmt.Fprintf(w, "プロキシの状態確認に失敗しました")
		return
	} else if !st {
		fmt.Println("切断済みです")
		w.WriteHeader(200)
		fmt.Fprintf(w, "切断済みです")
		return
	}

	_, err = exec.Command("iptables", "-A", "INPUT", "-p", "tcp", "--dport", "3128", "-j", "DROP").Output()
	if err != nil {
		fmt.Println("切断に失敗しました")
		w.WriteHeader(400)
		fmt.Fprintf(w, "切断に失敗しました")
		return
	}
	fmt.Println("切断ッッッッッ！！！")
	w.WriteHeader(200)
	fmt.Fprintf(w, "切断ッッッッッ！！！")
}

// プロキシ開始APIのハンドラ
// 実行すると全てのファイアウォールルールを消し去ることでプロキシを解放(ヨシッ！)
func connect(w http.ResponseWriter, r *http.Request) {
	fmt.Println("プロキシ開始要求を受信")
	_, err := exec.Command("iptables", "--flush").Output()
	if err != nil {
		fmt.Println("プロキシの開始に失敗しました")
		w.WriteHeader(400)
		fmt.Fprintf(w, "プロキシの開始に失敗しました")
		return
	}
	fmt.Println("プロキシの開始に成功しました")
	w.WriteHeader(200)
	fmt.Fprintf(w, "プロキシの開始に成功しました")
}

// プロキシの状態を確認するコマンドを実行する処理
func checkProxyCommand() (bool, error) {
	out, err := exec.Command("iptables", "--list", "INPUT").Output()
	if err != nil {
		return false, err
	}

	sStatus := string(out)

	// 3128ポートの拒否ルールがあるか確認(ヨシッ)
	index := strings.Index(sStatus, "3128")
	if index == -1 {
		return true, nil
	} else {
		return false, nil
	}
}

// プロキシの状態を確認するAPIのハンドラ
func checkPloxy(w http.ResponseWriter, r *http.Request) {
	fmt.Println("プロキシ状態確認要求を受信")
	st, err := checkProxyCommand()
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, "プロキシの状態確認に失敗しました")
		return
	}

	w.WriteHeader(200)
	if st {
		fmt.Fprintf(w, "true")
	} else {
		fmt.Fprintf(w, "false")
	}

}
