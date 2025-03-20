package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// CommandArg 命令行参数定义
type CommandArg struct {
	Type string `json:"type"`
}

// CommandConfig 命令行配置定义
type CommandConfig map[string]map[string]string

// 全局配置
var cmdConfig CommandConfig

// WebSocket 升级器
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求
	},
}

func main() {
	// 加载配置文件
	loadConfig()

	// 设置路由
	http.HandleFunc("/api/commands", getCommandsHandler)
	http.HandleFunc("/ws/execute", wsExecuteCommandHandler)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// 启动服务器
	log.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 加载配置文件
func loadConfig() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config/config.json" // 默认配置路径
	}

	loadConfigFile(configPath)

	// 每5秒检测配置文件是否变化
	go func() {
		var lastModTime time.Time
		for {
			time.Sleep(5 * time.Second)

			fileInfo, err := os.Stat(configPath)
			if err != nil {
				log.Printf("Failed to stat config file: %v", err)
				continue
			}

			currentModTime := fileInfo.ModTime()
			if currentModTime.After(lastModTime) && !lastModTime.IsZero() {
				log.Printf("Config file changed, reloading...")
				loadConfigFile(configPath)
			}

			lastModTime = currentModTime
		}
	}()
}

func loadConfigFile(configPath string) {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cmdConfig); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	log.Printf("Loaded commands: %v", cmdConfig)
}

// 获取所有命令配置
func getCommandsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if err := json.NewEncoder(w).Encode(cmdConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// WebSocket 执行命令处理器 - 用于实时输出
func wsExecuteCommandHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade to WebSocket:", err)
		return
	}
	defer conn.Close()

	// 接收命令请求
	var request struct {
		Cmd  string            `json:"cmd"`
		Args map[string]string `json:"args"`
	}

	if err := conn.ReadJSON(&request); err != nil {
		log.Println("Failed to read command request:", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to read command request"))
		return
	}

	// 检查命令是否存在于配置中
	cmdArgs, exists := cmdConfig[request.Cmd]
	if !exists {
		conn.WriteMessage(websocket.TextMessage, []byte("Command not found"))
		return
	}

	// 构建命令参数
	args := []string{}
	for argName, argType := range cmdArgs {
		if argValue, exists := request.Args[argName]; !exists || argValue == "" {
			continue
		}
		// 检查参数名是否包含[-x]格式
		if matches := regexp.MustCompile(`\[(.*)\]$`).FindStringSubmatch(argName); matches != nil {
			// 这是命令行格式的参数，如[-i]
			flagName := matches[1] // 提取-i部分
			argValue, exists := request.Args[argName]
			if !exists || argValue == "" {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Missing argument: %s", argName)))
				return
			}

			// 验证参数类型
			if argType == "int" {
				if _, err := strconv.Atoi(argValue); err != nil {
					http.Error(w, fmt.Sprintf("Argument %s must be an integer", flagName), http.StatusBadRequest)
					return
				}
			}

			args = append(args, flagName) // 添加参数名（如-i）
			args = append(args, argValue) // 添加参数值
		} else {
			// 普通参数处理
			argValue, exists := request.Args[argName]
			if !exists || argValue == "" {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Missing argument: %s", argName)))
				return
			}

			// 验证参数类型
			if argType == "int" {
				if _, err := strconv.Atoi(argValue); err != nil {
					conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Argument %s must be an integer", argName)))
					return
				}
			}

			args = append(args, argValue)
		}
	}

	// 执行命令
	cmd := exec.Command(request.Cmd, args...)
	// 将真正执行的指令输出到WebSocket
	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Executing command: %s %v", request.Cmd, args)))
	// 创建管道获取输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error creating stdout pipe: %v", err)))
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error creating stderr pipe: %v", err)))
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error starting command: %v", err)))
		return
	}

	// 读取并发送标准输出
	go func() {
		scanner := io.MultiReader(stdout, stderr)
		buf := make([]byte, 1024)
		for {
			n, err := scanner.Read(buf)
			if n > 0 {
				message := string(buf[:n])
				if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
					log.Println("Error writing to WebSocket:", err)
					return
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\nCommand execution failed: %v", err)))
	} else {
		conn.WriteMessage(websocket.TextMessage, []byte("\nCommand execution completed successfully"))
	}
}
