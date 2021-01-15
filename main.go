package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var port = flag.Int("port", 2222, "Run on this port")
var debug = flag.Bool("debug", false, "Enable debug output")

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

func getKeysFromGithub(username string) ([]string, error) {
	url := "https://github.com/" + username + ".keys"
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("GitHub user %v not found", username)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Status error: %v", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Read body: %v", err)
	}

	return strings.Split(string(data), "\n"), nil
}

func guessPublicIp() string {
	url := "https://api.ipify.org?format=text"
	resp, err := http.Get(url)
	if err != nil {
		return "<your ip>"
	}
	defer resp.Body.Close()
	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "<your ip>"
	}
	return string(ip)
}

func usage() {
	fmt.Println("Usage: sharessh <username>")
}

func main() {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	if *debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		config.EncoderConfig.EncodeCaller = nil
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	rawLogger, _ := config.Build()
	logger := rawLogger.Sugar()

	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	username := args[0]
	logger.Debugf("Fetching public SSH keys for %v", username)

	publicKeys, err := getKeysFromGithub(username)
	if err != nil {
		logger.Errorf("Error: %v", err)
		os.Exit(1)
	}

	ssh.Handle(func(s ssh.Session) {
		cmd := exec.Command("bash")
		ptyReq, winCh, isPty := s.Pty()
		if isPty {
			cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
			f, err := pty.Start(cmd)
			if err != nil {
				panic(err)
			}
			go func() {
				for win := range winCh {
					setWinsize(f, win.Width, win.Height)
				}
			}()
			go func() {
				io.Copy(f, s) // stdin
			}()
			io.Copy(s, f) // stdout
			cmd.Wait()
		} else {
			io.WriteString(s, "No PTY requested.\n")
			s.Exit(1)
		}
	})

	publicKeyOption := ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
		for keyNum, candidate := range publicKeys {
			logger.Debugf("Testing key: %v", candidate)
			pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(candidate))
			if err != nil {
				logger.Warnf("Skipping invalid public key %v", keyNum)
				continue
			}
			if ssh.KeysEqual(key, pk) {
				logger.Infof("@%v has connected!", username)
				return true
			}
		}
		logger.Warnf("Rejecting connection: No matching public key for @%v", username)
		return false
	})

	ip := guessPublicIp()

	logger.Infof("Running SSH server on port %v for @%v", *port, username)
	logger.Infof("Share this command:\n\n  ssh -o StrictHostKeyChecking=no -p %v %v\n", *port, ip)
	logger.Info("Press CTRL-C to quit.")
	listenAddr := fmt.Sprintf(":%v", *port)
	err = ssh.ListenAndServe(listenAddr, nil, publicKeyOption)
	if err != nil {
		logger.Fatalf("Error: %v", err)
	}
}
