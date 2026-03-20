package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Script 脚本结构
type Script struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Code        string    `json:"code"`
	Timeout     int64     `json:"timeout"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ScriptCommand 处理 script 相关命令
func ScriptCommand(args []string) {
	if len(args) == 0 {
		printScriptUsage()
		return
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "list", "ls":
		scriptList(subArgs)
	case "show", "get":
		scriptShow(subArgs)
	case "create", "new":
		scriptCreate(subArgs)
	case "update", "edit":
		scriptUpdate(subArgs)
	case "delete", "rm":
		scriptDelete(subArgs)
	default:
		fmt.Printf("未知命令: script %s\n", subCmd)
		printScriptUsage()
	}
}

func printScriptUsage() {
	fmt.Println(`
脚本仓库管理

用法:
  jpy-cloud script <命令> [参数]

命令:
  list                           列出所有脚本
  show <id>                      查看脚本详情
  create --name <名称> --code <代码> [--code-file <文件>] [--desc <描述>] [--timeout 60000]
  update <id> [--name <名称>] [--code <代码>] [--code-file <文件>] [--desc <描述>] [--timeout 60000]
  delete <id>                    删除脚本

选项:
  --code       直接传入代码内容
  --code-file  从文件读取代码内容（与 --code 二选一）
  --json       JSON 格式输出

示例:
  jpy-cloud script list
  jpy-cloud script create --name "截图测试" --code "return screenshot()"
  jpy-cloud script create --name "复杂脚本" --code-file ./my_script.js
  jpy-cloud script update 1 --code "return 1+1" --timeout 30000
  jpy-cloud script delete 1`)
}

// parseScriptFlags 解析脚本命令的通用参数
func parseScriptFlags(args []string) (name, code, desc string, timeout int64, jsonOutput bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--code":
			if i+1 < len(args) {
				code = args[i+1]
				i++
			}
		case "--code-file":
			if i+1 < len(args) {
				data, err := os.ReadFile(args[i+1])
				if err != nil {
					fmt.Printf("读取文件失败: %v\n", err)
					os.Exit(1)
				}
				code = string(data)
				i++
			}
		case "--desc":
			if i+1 < len(args) {
				desc = args[i+1]
				i++
			}
		case "--timeout":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &timeout)
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}
	return
}

func scriptList(args []string) {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		}
	}

	var scripts []Script
	if err := rpaGet("/scripts", &scripts); err != nil {
		fmt.Printf("获取脚本列表失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(scripts)
		return
	}

	if len(scripts) == 0 {
		fmt.Println("暂无脚本")
		return
	}

	fmt.Printf("%-6s %-20s %-30s %-10s %-20s\n", "ID", "名称", "描述", "超时(ms)", "更新时间")
	fmt.Println(strings.Repeat("-", 90))
	for _, s := range scripts {
		desc := s.Description
		if len(desc) > 28 {
			desc = desc[:28] + ".."
		}
		fmt.Printf("%-6d %-20s %-30s %-10d %-20s\n",
			s.ID, s.Name, desc, s.Timeout, s.UpdatedAt.Format("2006-01-02 15:04"))
	}
}

func scriptShow(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud script show <id> [--json]")
		return
	}

	id := args[0]
	jsonOutput := false
	for _, a := range args[1:] {
		if a == "--json" {
			jsonOutput = true
		}
	}

	var script Script
	if err := rpaGet("/scripts/"+id, &script); err != nil {
		fmt.Printf("获取脚本失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(script)
		return
	}

	fmt.Printf("ID:      %d\n", script.ID)
	fmt.Printf("名称:    %s\n", script.Name)
	fmt.Printf("描述:    %s\n", script.Description)
	fmt.Printf("超时:    %dms\n", script.Timeout)
	fmt.Printf("创建:    %s\n", script.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("更新:    %s\n", script.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("代码:\n%s\n", script.Code)
}

func scriptCreate(args []string) {
	name, code, desc, timeout, jsonOutput := parseScriptFlags(args)

	if name == "" {
		fmt.Println("错误: --name 必填")
		return
	}
	if code == "" {
		fmt.Println("错误: --code 或 --code-file 必填")
		return
	}
	if timeout == 0 {
		timeout = 60000
	}

	body := map[string]interface{}{
		"name":        name,
		"code":        code,
		"description": desc,
		"timeout":     timeout,
	}

	var script Script
	if err := rpaPost("/scripts", body, &script); err != nil {
		fmt.Printf("创建脚本失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(script)
	} else {
		fmt.Printf("✓ 脚本已创建: %s (ID: %d)\n", script.Name, script.ID)
	}
}

func scriptUpdate(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud script update <id> [--name ...] [--code ...] [--code-file ...] [--desc ...] [--timeout ...]")
		return
	}

	id := args[0]
	name, code, desc, timeout, jsonOutput := parseScriptFlags(args[1:])

	if name == "" && code == "" && desc == "" && timeout == 0 {
		fmt.Println("错误: 至少指定一个要更新的字段")
		return
	}

	// 先获取当前脚本，做增量合并
	var current Script
	if err := rpaGet("/scripts/"+id, &current); err != nil {
		fmt.Printf("获取当前脚本失败: %v\n", err)
		return
	}

	// 只覆盖用户指定的字段，其余保留原值
	body := map[string]interface{}{
		"name":        current.Name,
		"code":        current.Code,
		"description": current.Description,
		"timeout":     current.Timeout,
	}
	if name != "" {
		body["name"] = name
	}
	if code != "" {
		body["code"] = code
	}
	if desc != "" {
		body["description"] = desc
	}
	if timeout > 0 {
		body["timeout"] = timeout
	}

	var script Script
	if err := rpaPut("/scripts/"+id, body, &script); err != nil {
		fmt.Printf("更新脚本失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(script)
	} else {
		fmt.Printf("✓ 脚本已更新: %s (ID: %s)\n", script.Name, id)
	}
}

func scriptDelete(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud script delete <id>")
		return
	}

	id := args[0]

	var script Script
	if err := rpaGet("/scripts/"+id, &script); err != nil {
		fmt.Printf("获取脚本失败: %v\n", err)
		return
	}

	if err := rpaDeleteRequest("/scripts/" + id); err != nil {
		fmt.Printf("删除脚本失败: %v\n", err)
		return
	}

	// 检查是否 --json
	jsonOutput := false
	for _, a := range args[1:] {
		if a == "--json" {
			jsonOutput = true
		}
	}

	if jsonOutput {
		OutputJSON(map[string]interface{}{"success": true, "id": id, "name": script.Name})
	} else {
		fmt.Printf("✓ 已删除脚本: %s (ID: %s)\n", script.Name, id)
	}
}
