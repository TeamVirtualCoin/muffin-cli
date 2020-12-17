package main

import(
	"net/http"
	"io/ioutil"
	"bytes"
	"fmt"
	"encoding/json"
	"time"
	"strconv"
	"github.com/gorilla/mux"
	"github.com/robertkrimen/otto"
	"github.com/teamvirtualcoin/libhashx"
)

var port = "3000"
var mnemonics = []string{"dog","cat","tiger","lion","elephant","crocodile","rabbit","rat","chicken",
	"cheetah","puma","alligator","cow","buffalo","dinosaur","cockroach","trees"}
var hashx = libhashx.LibHashX{Mnemonic : mnemonics,Length : 20}

type Transaction struct {
	Txid int `storm:"id,increment" json:"txid"`
	Amount float64 `json:"amount"`
	Sender string `json:"sender"`
	Receiver string `json:"receiver"`
	Timestamp float64 `storm:"index" json:"timestamp"`
	Txtype string `json:"txtype"`
	Code string `json:"code"`
}

type jsonTx struct {
	PrivateKey string `json:"privateKey"`
	Receiver string `json:"receiver"`
	Amount float64 `json:"amount"`
}

type jsonSupply struct {
	PrivateKey string `json:"privateKey"`
	Amount float64 `json:"amount"`
	Txtype string `json:"txtype"`
}

type jsonSc struct {
	PrivateKey string `json:"privateKey"`
	Code string `json:"code"`
}

type jsonCall struct {
	Txid int `json:txid`
	PrivateKey string `json:privateKey`
	Call string `json:call`
}

type LibHttp struct {
	Get func(string) (string,string)
	Post func(string,string,string) (string,string)
}

type Core struct {
	Set func(interface{},interface{})
	SetUser func(string,interface{},interface{})
	Get func(interface{}) interface{}
	GetUser func(string,interface{}) interface{}
	LibHttp LibHttp
}

type KeyValue struct {
	Key interface{}
	Value interface{}
}

var txdb []Transaction
var coindb []KeyValue
var counter int = 0

func Faucet(publicKey string) {
	thistx := Transaction{
		Txid : counter + 1,
		Amount : 100,
		Receiver : publicKey,
		Sender : "muffin-cli",
		Timestamp : float64(int(time.Now().UnixNano()) / 1e6),
		Txtype : "faucet",
	}
	counter += 1
	txdb = append(txdb,thistx)
}

func coindbset(bucket string,key interface{},value interface{}) {
	var b interface{} = bucket
	for z,i := range coindb {
		if i.Key == b.(string) + key.(string) {
			coindb[z] = KeyValue{Key : b.(string) + key.(string),Value : value}
			return
		}
	}
	coindb = append(coindb,KeyValue{Key : b.(string) + key.(string),Value : value})
}

func coindbget(bucket string,key interface{}) interface{} {
	var b interface{} = bucket
	for _,i := range coindb {
		if i.Key == b.(string) + key.(string) {
			return i.Value
		}
	}
	return nil
}

func CreateWallet() []string {
	key := hashx.GenPriv()
	publicKey := hashx.GenPub(key[0])
	Faucet(publicKey)
	return []string{key[1],key[0],publicKey}
}

func MnemonicToPrivate(mnemonic string) string {
	return libhashx.Hash(mnemonic)
}

func PrivateToPublic(privateKey string) string {
	return hashx.GenPub(privateKey)
}

func GetBal(publicKey string) float64 {
	var inputs []Transaction
	var outputs []Transaction
	for _,i := range txdb {
		if i.Receiver == publicKey {
			inputs = append(inputs,i)
			continue
		}
		if i.Sender == publicKey {
			outputs = append(outputs,i)
			continue
		}
	}
	var ia float64
	var oa float64
	for _,i := range inputs {
		ia += i.Amount
		continue
	}
	for _,k := range outputs {
		oa += k.Amount
		continue
	}
	if oa > ia {return 0}
	bal := ia - oa
	return bal
}

func Mint(privateKey string,amount float64) interface{} {
	pubKey := hashx.GenPub(privateKey)
	thistx := Transaction{
		Txid : counter + 1,
		Amount : amount,
		Sender : "supply",
		Receiver : pubKey,
		Timestamp : float64(int(time.Now().UnixNano()) / 1e6),
		Txtype : "mint",
	}
	counter += 1
	txdb = append(txdb,thistx)
	return "true"
}

func Burn(privateKey string,amount float64) interface{} {
	pubKey := hashx.GenPub(privateKey)
    if GetBal(pubKey) < amount {
    	return false
    }
    thistx := Transaction{
    	Txid : counter + 1,
        Amount : amount,
        Sender : pubKey,
        Receiver : "supply",
        Timestamp : float64(int(time.Now().UnixNano()) / 1e6),
        Txtype : "burn",
    }
    counter += 1
    txdb = append(txdb,thistx)
    return "true"
}

func SendTx(privateKey string,amount float64,receiver string) string {
	pubKey := hashx.GenPub(privateKey)
	bal := GetBal(pubKey)
	if bal - amount < 0 {
		return "error"
	}
	if amount < 0.0001 {
		return "error"
	}
	if len(privateKey) != 64 && len(receiver) != 64 {
		return "error"
	}
	thistx := Transaction{
		Txid : counter + 1,
		Amount : amount,
		Sender : pubKey,
		Receiver : receiver,
		Timestamp : float64(int(time.Now().UnixNano()) / 1e6),
		Txtype : "normal",
	}
	ak,zv := json.Marshal(thistx)
	if zv != nil {
		return "error"
	}
	txdb = append(txdb,thistx)
	counter += 1
	return string(ak)
}

func EstimateContractFuel(code string) float64 {
	return float64(len(code)) * 0.0001
}

func SendContract(privateKey string,code string) string {
	amount := EstimateContractFuel(code)
	pubKey := hashx.GenPub(privateKey)
	bal := GetBal(pubKey)
	if bal - amount <= 0 {
		return "error"
	}
	if len(privateKey) != 64 && len(code) <= 0 {
		return "error"
	}
	thistx := Transaction{
		Txid : counter + 1,
		Amount : amount,
		Sender : pubKey,
		Receiver : "3ef416197407a4324f961bd6f2dad2e003f7c8531ee261af2c5ca9e382b11483",
		Timestamp : float64(int(time.Now().UnixNano()) / 1e6),
		Txtype : "contract",
		Code : code,
	}
	ak,zv := json.Marshal(thistx)
	if zv != nil {
		return "error"
	}
	status := make(chan string)
	go CallFunc(counter + 1,privateKey,"_initialize()",status)
	select {
		case call := <-status:
	    	if call == "error" {
	        	return "error"
	        }
	    case <-time.After(20 * time.Second):
	            return "error"
	}
	txdb = append(txdb,thistx)
	counter += 1
	return string(ak)
}

func IsContract(txid int) bool {
	var tx Transaction
	err := json.Unmarshal([]byte(GetTxById(txid)),&tx)
	if err != nil {
		return false
	}
	if tx.Txtype == "contract" {
		return true
	}
	return false
}

func CallFunc(txid int,privateKey string,call string,value chan string) {
	pubKey := hashx.GenPub(privateKey)
	if IsContract(txid) == false {
		value <- "error"
		return
	}
	var tx Transaction
	err := json.Unmarshal([]byte(GetTxById(txid)),&tx)
	if err != nil {
		value <- "error"
		return
	}
	id := strconv.Itoa(txid)
	libHttp := LibHttp{
		Get : func (url string) (string,string) {
			client := http.Client{
				Timeout : 4 * time.Second,
			}
			res, errclient := client.Get(url)
			if errclient != nil {
				return "error", "error"
			}
			defer res.Body.Close()
			body, errparse := ioutil.ReadAll(res.Body)
			if errparse != nil {
				return "error", "error"
			}
			return string(body), res.Status
		},
		Post : func(url string,contentType string,body string) (string,string) {
			client := http.Client{
				Timeout : 4 * time.Second,
			}
			res, errclient := client.Post(url,contentType,bytes.NewBuffer([]byte(body)))
			if errclient != nil {
				return "error", "error"
			}
			defer res.Body.Close()
			resbody, errparse := ioutil.ReadAll(res.Body)
			if errparse != nil {
				return "error", "error"
			}
			return string(resbody), res.Status
		},
	}
	core := Core{
		Set : func (variable interface{},value interface{}) {
		    coindbset(id,variable,value)
		},
		SetUser : func (user string,variable interface{},value interface{}) {
		    coindbset(id + user,variable,value)
		},
		Get : func (variable interface{}) interface{} {
			var value interface{} = coindbget(id,variable)
			return value
		},
		GetUser : func (user string,variable interface{}) interface{} {
			var value interface{} = coindbget(id + user,variable)
			return value
		},
		LibHttp : libHttp,
	}
	vm := otto.New()
	vm.Run(tx.Code)
	vm.Set("deployer",tx.Sender)
	vm.Set("sender",pubKey)
	vm.Set("core",core)
	str,err3 := vm.Run(call)
	if err3 != nil {
		value <- "error"
		return
	}
	str2,err4 := str.ToString()
	if err4 != nil {
		value <- "error"
		return
	}
	value <- str2
}

func GetTxById(id int) string {
	var txbyid Transaction
	for _,i := range txdb {
		if i.Txid == id {
			txbyid = i
		}
	}
	txd, erken := json.Marshal(txbyid)
	if erken != nil{
		return "error"
	}
	return string(txd)
}

func TotalReceivedTx(publicKey string) string {
	var inputs []Transaction
	for _,i := range txdb {
		if i.Receiver == publicKey {
			inputs = append(inputs,i)
		}
	}
	jzon, erzkene := json.Marshal(inputs)
	if erzkene != nil {
		return "error"
	}
	return string(jzon)
}

func TotalSentTx(publicKey string) string {
	var outputs []Transaction
	for _,i := range txdb {
		if i.Sender == publicKey {
			outputs = append(outputs,i)
		}
	}
	jzon, erzkene := json.Marshal(outputs)
	if erzkene != nil {
		return "error"
	}
	return string(jzon)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/createwallet",func(res http.ResponseWriter,req *http.Request){
		nwallet := CreateWallet()
		wallet := map[string]string{"mnemonic" : nwallet[0],"privateKey" : nwallet[1],"publicKey" : nwallet[2]}
		jxon, errvf := json.Marshal(wallet)
		if errvf != nil {
			http.Error(res,"Error While Creating A Wallet, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
		fmt.Fprintf(res,string(jxon))
	}).Methods("GET")
	r.HandleFunc("/sendtx",func(res http.ResponseWriter,req *http.Request) {
		var tx jsonTx
		errkn := json.NewDecoder(req.Body).Decode(&tx)
		if errkn != nil {
			http.Error(res,"Error While Parsing Your Transaction Body, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		oktx := SendTx(tx.PrivateKey,tx.Amount,tx.Receiver)
		if oktx == "error" {
			http.Error(res,"Error While Sending Your Transaction, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
		fmt.Fprintf(res,oktx)
	}).Methods("POST")
	r.HandleFunc("/editsupply",func(res http.ResponseWriter,req *http.Request) {
        var tx jsonSupply
        errkn := json.NewDecoder(req.Body).Decode(&tx)
        if errkn != nil {
            http.Error(res,"Error While Parsing Your Transaction Body, Retry Or Contact The Team",http.StatusBadRequest)
            return
        }
        var oktx interface{}
        if tx.Txtype == "mint" {
        	oktx = Mint(tx.PrivateKey,tx.Amount)
        } else if tx.Txtype == "burn" {
        	oktx = Burn(tx.PrivateKey,tx.Amount)
        }
        if oktx == false || oktx == nil {
            http.Error(res,"Error While Sending Your Transaction, Retry Or Contact The Team",http.StatusBadRequest)
            return
        }
        res.WriteHeader(http.StatusOK)
        fmt.Fprintf(res,"true")
    }).Methods("POST")
	r.HandleFunc("/sendcontract",func(res http.ResponseWriter,req *http.Request) {
		var tx jsonSc
		errkn := json.NewDecoder(req.Body).Decode(&tx)
		if errkn != nil {
			http.Error(res,"Error While Parsing Your Contract, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		oktx := SendContract(tx.PrivateKey,tx.Code)
		if oktx == "error" {
			http.Error(res,"Error While Sending Your Contract, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
		fmt.Fprintf(res,oktx)
	}).Methods("POST")
	r.HandleFunc("/callcontract",func(res http.ResponseWriter,req *http.Request) {
		var fc jsonCall
		errkn := json.NewDecoder(req.Body).Decode(&fc)
		if errkn!= nil {
			http.Error(res,"Error While Parsing Your Call, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		if string(fc.Call[0]) == "_" {
			http.Error(res,"Unable To Call Private Function",http.StatusBadRequest)
			return
		}
		status := make(chan string)
		go CallFunc(fc.Txid,fc.PrivateKey,fc.Call,status)
		select {
			case call := <-status:
				if call == "error" {
					http.Error(res,"Error While Executing Your Call, Retry Or Contact The Team",http.StatusBadRequest)
					return
				}
				res.WriteHeader(http.StatusOK)
				fmt.Fprintf(res,call)
				return
			case <-time.After(20 * time.Second):
				http.Error(res,"Function Takes Too Much Time, Timeout, Retry Or Contact The Token Team/Team",http.StatusBadRequest)
				return
		}
	}).Methods("POST")
	r.HandleFunc("/iscontract/{txid}",func(res http.ResponseWriter,req *http.Request) {
		tx := mux.Vars(req)["txid"]
		id,err := strconv.Atoi(tx)
		var a string
		if IsContract(id) {
			a = "true"
		} else {
			a = "false"
		}
		if err != nil {
			http.Error(res,"Please Only Include Integers/Numbers, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
		fmt.Fprintf(res,a)
	})
	r.HandleFunc("/contractfuel/{code}",func(res http.ResponseWriter,req *http.Request) {
		code := mux.Vars(req)["code"]
		res.WriteHeader(http.StatusOK)
		fmt.Fprintf(res,fmt.Sprintf("%f",EstimateContractFuel(code)))
	}).Methods("GET")
	r.HandleFunc("/gettx/{id}",func(res http.ResponseWriter,req *http.Request) {
		id := mux.Vars(req)["id"]
		a,j := strconv.Atoi(id)
		if j != nil {
			http.Error(res,"Please Only Include Integers/Numbers, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		txc := GetTxById(a)
		if txc == "error" {
			http.Error(res,"This Transaction Doesnt Exist/Error, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
		fmt.Fprintf(res,txc)
	}).Methods("GET")
	r.HandleFunc("/balance/{address}",func(res http.ResponseWriter,req *http.Request) {
		address := mux.Vars(req)["address"]
		fmt.Fprintf(res,fmt.Sprintf("%f",GetBal(address)))
	}).Methods("GET")
	r.HandleFunc("/receivedtx/{address}",func(res http.ResponseWriter,req *http.Request) {
		address := mux.Vars(req)["address"]
		txs := TotalReceivedTx(address)
		if txs == "error" {
			http.Error(res,"Error Cant Check Received Txs, Retry Or Contact The Team",http.StatusBadRequest)
			return
		}
		res.WriteHeader(http.StatusOK)
		fmt.Fprintf(res,string(txs))
	}).Methods("GET")
	r.HandleFunc("/senttx/{address}",func(res http.ResponseWriter,req *http.Request) {
	    address := mux.Vars(req)["address"]
	    txs := TotalSentTx(address)
	    if txs == "error" {
	        http.Error(res,"Error Cant Check Sent Txs, Retry Or Contact The Team",http.StatusBadRequest)
	        return
	    }
	    res.WriteHeader(http.StatusOK)
	    fmt.Fprintf(res,(string(txs)))
	}).Methods("GET")
	green := "\033[32m"
	fmt.Println(green," -----------------------------------------")
	fmt.Println(green," |Running On Private Testnet : Muffin Cli|")
	fmt.Println(green," |Version : v0.0.8 BETA                  |")
	fmt.Println(green," |By TeamVirtualCoin                     |")
	fmt.Println(green," |---------------------------------------|")
	fmt.Println(green," |VirtualCoin - The First VirtualCurrency|")
	fmt.Println(green," |By TeamVirtualCoin                     |")
	fmt.Println(green," |---------------------------------------|")
	fmt.Println(green," |Listening On Port " + port + "                 |")
	fmt.Println(green," -----------------------------------------")
	http.ListenAndServe(":" + port,r)
}
