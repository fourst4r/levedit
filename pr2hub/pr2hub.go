package pr2hub

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

var (
	loginKey = []byte{85, 74, 47, 106, 110, 70, 42, 119, 82, 48, 113, 82, 75, 47, 100, 72}
	loginIV  = []byte{38, 99, 57, 42, 121, 42, 53, 112, 61, 49, 85, 78, 120, 47, 84, 114}
)

func encrypt(src string, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	cbc := cipher.NewCBCEncrypter(block, loginIV)
	content := zerosPad([]byte(src), cbc.BlockSize())
	crypted := make([]byte, len(content))
	cbc.CryptBlocks(crypted, content)
	return crypted, nil
}

// Pad the ciphertext with zeros until it is of length blockSize.
func zerosPad(ciphertext []byte, blockSize int) []byte {
	// determine number of zeros to add
	padLen := blockSize - (len(ciphertext) % blockSize)
	padText := bytes.Repeat([]byte{0}, padLen)
	ciphertext = append(ciphertext, padText...)
	return ciphertext
}

// Trim the trailing zeros from ciphertext.
func zerosUnpad(ciphertext []byte) []byte {
	return bytes.TrimRight(ciphertext, string(byte(0)))
}

type LoginResponse struct {
	Success        bool        `json:"success"`
	Error          string      `json:"error"`
	Message        interface{} `json:"message"` // always null?
	UserID         int         `json:"userId"`
	Token          string      `json:"token"`
	Email          bool        `json:"email"`
	Ant            bool        `json:"ant"`
	Time           int         `json:"time"`
	LastRead       string      `json:"lastRead"`
	LastRecv       interface{} `json:"lastRecv"`
	Guild          string      `json:"guild"`
	GuildOwner     int         `json:"guildOwner"`
	GuildName      string      `json:"guildName"`
	Emblem         string      `json:"emblem"`
	FavoriteLevels []int       `json:"favoriteLevels"`
}

const (
	build   = "22-jun-2020-v160"
	referer = "https://pr2hub.com/"
)

func Login(user, pass string, remember bool) (*Req, error) {
	const j = `{
		"build":"%s",
		"domain":"pr2hub.com",
		"login_id":12985,
		"remember":%t,
		"user_name":"%s",
		"user_pass":"%s",
		"server":{
			"port":9160,
			"status":"open",
			"server_id":1,
			"happy_hour":0,
			"server_name":"Derron",
			"address":"45.76.24.255",
			"guild_id":0,
			"tournament":"0",
			"population":40
		}
	}`
	b, err := encrypt(fmt.Sprintf(j, build, remember, user, pass), loginKey)
	if err != nil {
		return nil, err
	}
	i := base64.RawStdEncoding.EncodeToString(b)

	form := make(url.Values)
	form.Add("build", build)
	form.Add("i", i)
	req, err := http.NewRequest("POST", "https://pr2hub.com/login.php", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Referer", referer)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var r LoginResponse
	return httpReq2(req, &r), nil
}

func UploadLevel(data string) (*Req, error) {
	req, err := http.NewRequest("POST", "https://pr2hub.com/upload_level.php", strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	values := make(url.Values)
	return httpReq3(req, queryUnmarshal, &values), nil
}

type DeleteLevelResponse jsonResponse

func DeleteLevel(levelID, token string) (*Req, error) {
	body := make(url.Values)
	body.Set("level_id", levelID)
	body.Set("token", token)
	req, err := http.NewRequest("POST", "https://pr2hub.com/delete_level.php", strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Referer", referer)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var d DeleteLevelResponse
	return httpReq3(req, jsonUnmarshal, &d), nil
}

type CheckLoginResponse struct {
	UserName string      `json:"user_name"`
	GuildID  interface{} `json:"guild_id"`
}

// func CheckLogin() (*CheckLoginResponse, error) {
// 	resp, err := http.Get("https://pr2hub.com/check_login.php")
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()
// 	var r CheckLoginResponse
// 	err = json.NewDecoder(resp.Body).Decode(&r)
// 	return &r, err
// }

type LevelsGetResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Levels  []struct {
		LevelID   string  `json:"level_id"`
		Version   string  `json:"version"`
		Title     string  `json:"title"`
		Rating    float64 `json:"rating"`
		PlayCount string  `json:"play_count"`
		MinLevel  string  `json:"min_level"`
		Note      string  `json:"note"`
		Live      string  `json:"live"`
		Type      string  `json:"type"`
		Time      string  `json:"time"`
		Name      string  `json:"name"`
		Power     string  `json:"power"`
		TrialMod  string  `json:"trial_mod"`
		UserID    string  `json:"user_id"`
	} `json:"levels"`
}

func LevelsGet() *Req {
	// var l LevelsGetResponse
	// return httpReq(&l, "GET", "https://pr2hub.com/levels_get.php", nil)

	req, err := http.NewRequest("GET", "https://pr2hub.com/levels_get.php", nil)
	if err != nil {
		log.Fatal(err)
	}
	var l LevelsGetResponse
	return httpReq3(req, jsonUnmarshal, &l)
}

// TODO: v exists so the unmarshalling happens in another goroutine,
//       is this necessary?
func httpReq3(request *http.Request, unmarshal UnmarshalFunc, v interface{}) *Req {
	respCh := make(chan interface{})
	ctx, cancel := context.WithCancel(context.Background())
	req := &Req{ctx: ctx, cancel: cancel, respCh: respCh}
	method, url := request.Method, request.URL.String()
	wrap := func(err error) error {
		return errors.Wrap(err, fmt.Sprintf("ERR %s %q", method, url))
	}
	log.Println("} BEGIN {", method, url)

	go func() {
		defer close(respCh)
		defer cancel()
		defer log.Println("} CANCEL {", method, url)

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			respCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respCh <- wrap(fmt.Errorf("bad status: %s", resp.Status))
			return
		}

		err = unmarshal(resp.Body, v)
		if err != nil {
			respCh <- err
			return
		}
		respCh <- v

		log.Println("} SUCCESS {", method, url)
	}()

	return req
}

func httpReq2(request *http.Request, v interface{}) *Req {
	respCh := make(chan interface{})
	ctx, cancel := context.WithCancel(context.Background())
	req := &Req{ctx: ctx, cancel: cancel, respCh: respCh}
	method, url := request.Method, request.URL.String()
	wrap := func(err error) error {
		return errors.Wrap(err, fmt.Sprintf("ERR %s %q", method, url))
	}
	log.Println("} BEGIN {", method, url)

	go func() {
		defer close(respCh)
		defer cancel()
		defer log.Println("} CANCEL {", method, url)

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			respCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respCh <- wrap(fmt.Errorf("bad status: %s", resp.Status))
			return
		}

		isJSON := (v != nil)
		if isJSON {
			err = json.NewDecoder(resp.Body).Decode(v)
			if err != nil {
				respCh <- err
				return
			}
			respCh <- v
		} else {
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				respCh <- err
				return
			}
			respCh <- string(b)
		}
		log.Println("} SUCCESS {", method, url)
	}()

	return req
}

// func httpNoReq(ctx context.Context, request *http.Request, v interface{}) func(interface{}, *error) bool {
// 	respCh := make(chan interface{})
// 	ctx, cancel := context.WithCancel(context.Background())
// 	// req2 := &Req2{ctx: ctx, cancel: cancel, respCh: respCh}
// 	method, url := request.Method, request.URL.String()
// 	wrap := func(err error) error {
// 		return errors.Wrap(err, fmt.Sprintf("ERR %s %q", method, url))
// 	}
// 	log.Println("} BEGIN {", method, url)

// 	go func() {
// 		defer close(respCh)
// 		defer cancel()
// 		defer log.Println("} CANCEL {", method, url)

// 		resp, err := http.DefaultClient.Do(request)
// 		if err != nil {
// 			respCh <- err
// 			return
// 		}
// 		defer resp.Body.Close()

// 		if resp.StatusCode != http.StatusOK {
// 			respCh <- wrap(fmt.Errorf("bad status: %s", resp.Status))
// 			return
// 		}

// 		isJSON := (v != nil)
// 		if isJSON {
// 			err = json.NewDecoder(resp.Body).Decode(v)
// 			if err != nil {
// 				respCh <- err
// 				return
// 			}
// 			respCh <- v
// 		} else {
// 			b, err := ioutil.ReadAll(resp.Body)
// 			if err != nil {
// 				respCh <- err
// 				return
// 			}
// 			respCh <- string(b)
// 		}
// 		log.Println("} SUCCESS {", method, url)
// 	}()

// 	return func(v interface{}, errp *error) bool {
// 		select {
// 		case <-ctx.Done():
// 			// request was canceled or timed out
// 			// r.err = ctx.Err()
// 			return true
// 		case resp, notClosed := <-respCh:
// 			if notClosed {
// 				if err, ok := resp.(error); ok {
// 					*errp = err
// 					return true
// 				}
// 				// http request succeeded!
// 				rv := reflect.ValueOf(v)
// 				if rv.Kind() != reflect.Ptr || rv.IsNil() {
// 					panic("v should be non-nil pointer")
// 				}
// 				// *v = *resp
// 				if reflect.ValueOf(resp).Kind() == reflect.String {
// 					rv.Elem().SetString(resp.(string))
// 				} else {
// 					rv.Elem().Set(reflect.ValueOf(resp).Elem())
// 				}
// 				return true
// 			}
// 			// we already succeeded, but ok
// 			return true
// 		default:
// 			// still not done
// 			return false
// 		}
// 	}
// }

func httpReq(v interface{}, method string, url string, body io.Reader) *Req {
	respCh := make(chan interface{})
	ctx, cancel := context.WithCancel(context.Background())
	req := &Req{ctx: ctx, cancel: cancel, respCh: respCh}
	wrap := func(err error) error {
		return errors.Wrap(err, fmt.Sprintf("ERR %s %q", method, url))
	}
	log.Println("} BEGIN {", method, url)

	go func() {
		defer close(respCh)
		defer cancel()
		defer log.Println("} CANCEL {", method, url)

		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			respCh <- err
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			respCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respCh <- wrap(fmt.Errorf("bad status: %s", resp.Status))
			return
		}

		isJSON := (v != nil)
		if isJSON {
			err = json.NewDecoder(resp.Body).Decode(v)
			if err != nil {
				respCh <- err
				return
			}
			respCh <- v
		} else {
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				respCh <- err
				return
			}
			respCh <- string(b)
		}
		log.Println("} SUCCESS {", method, url)
	}()

	return req
}

func Level(id, version string) *Req {
	uri := fmt.Sprintf("https://pr2hub.com/levels/%s.txt?version=%s", id, version)
	// return httpReq(nil, "GET", url, nil)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		log.Fatal(err)
	}
	// query := make(url.Values)
	var data string
	// levels are not correctly url escaped so we can't
	// unmarshal to url.Values
	return httpReq3(req, stringUnmarshal, &data)
}

type Req struct {
	ctx    context.Context
	cancel context.CancelFunc
	respCh <-chan interface{}
	err    error
}

func (r *Req) Done(v interface{}) bool {
	select {
	case <-r.ctx.Done():
		// request was canceled or timed out
		r.err = r.ctx.Err()
		return true
	case resp, notClosed := <-r.respCh:
		if notClosed {
			if err, ok := resp.(error); ok {
				r.err = err
				return true
			}
			// http request succeeded!
			rv := reflect.ValueOf(v)
			if rv.Kind() != reflect.Ptr || rv.IsNil() {
				panic("v should be non-nil pointer")
			}
			// *v = *resp
			if reflect.ValueOf(resp).Kind() == reflect.String {
				rv.Elem().SetString(resp.(string))
			} else {
				rv.Elem().Set(reflect.ValueOf(resp).Elem())
			}
			return true
		}
		// we already succeeded, but ok
		return true
	default:
		// still not done
		return false
	}
}

func (r *Req) Cancel() {
	r.cancel()
}

func (r *Req) Err() error {
	return r.err
}

type jsonResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type UnmarshalFunc func(r io.Reader, v interface{}) error

var (
	jsonUnmarshal = func(r io.Reader, v interface{}) error {
		return json.NewDecoder(r).Decode(v)
	}
	stringUnmarshal = func(r io.Reader, v interface{}) error {
		b, err := ioutil.ReadAll(r)
		*v.(*string) = string(b)
		return err
	}
	queryUnmarshal = func(r io.Reader, v interface{}) error {
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		*v.(*url.Values), err = url.ParseQuery(string(b))
		return err
	}
)
