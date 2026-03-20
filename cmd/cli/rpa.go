package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RpaFlow RPA 流程结构
type RpaFlow struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Steps       []RpaStep `json:"steps"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// RpaStep RPA 步骤结构
type RpaStep struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params"`
}

// DeviceConfig 设备配置结构
type DeviceConfig struct {
	DeviceID       int        `json:"deviceId"`
	RpaID          uint       `json:"rpaId"`
	RpaName        string     `json:"rpaName"`
	Status         string     `json:"status"`
	Mode           string     `json:"mode"`
	CurrentStep    int        `json:"currentStep"`
	TotalSteps     int        `json:"totalSteps"`
	StepName       string     `json:"stepName"`
	SubStep        int        `json:"subStep"`
	SubStepName    string     `json:"subStepName"`
	LoopCount      int        `json:"loopCount"`
	SuccessCount   int        `json:"successCount"`
	FailCount      int        `json:"failCount"`
	TotalTime      int        `json:"totalTime"`
	LastError      string     `json:"lastError"`
	StartedAt      *time.Time `json:"startedAt"`
	LoopStartAt    *time.Time `json:"loopStartAt"`
	ScriptStatus   string     `json:"scriptStatus"`
	ScriptProgress int        `json:"scriptProgress"`
}

// ExecutionHistory 执行历史结构
type ExecutionHistory struct {
	ID          uint      `json:"id"`
	DeviceID    int       `json:"deviceId"`
	RpaID       uint      `json:"rpaId"`
	RpaName     string    `json:"rpaName"`
	Status      string    `json:"status"`
	CurrentStep int       `json:"currentStep"`
	TotalSteps  int       `json:"totalSteps"`
	Duration    int       `json:"duration"`
	Result      string    `json:"result"`
	ErrorMsg    string    `json:"errorMsg"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
}

// RpaCommand 处理 RPA 相关命令
func RpaCommand(args []string) {
	if len(args) == 0 {
		printRpaUsage()
		return
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "list", "ls":
		rpaList(subArgs)
	case "show", "get":
		rpaShow(subArgs)
	case "create", "new":
		rpaCreate(subArgs)
	case "delete", "rm":
		rpaDeleteCmd(subArgs)
	case "step":
		rpaStep(subArgs)
	case "run", "exec":
		rpaRun(subArgs)
	case "status":
		rpaStatus(subArgs)
	case "history":
		rpaHistory(subArgs)
	case "stop":
		rpaStop(subArgs)
	default:
		fmt.Printf("未知命令: rpa %s\n", subCmd)
		printRpaUsage()
	}
}

func printRpaUsage() {
	fmt.Println(`
RPA 命令行工具

用法:
  jpy-cloud rpa <命令> [参数]

流程管理:
  list                           列出所有 RPA 流程
  show <id>                      查看 RPA 详情
  create --name <名称> [--desc <描述>]  创建 RPA 流程
  delete <id>                    删除 RPA 流程

步骤管理:
  step list <rpa_id>             列出 RPA 的所有步骤
  step add <rpa_id> --type <类型> --name <名称> [--params <JSON>]  添加步骤
  step edit <rpa_id> <index> [--name <名称>] [--set key=value] [--unset key] [--params <JSON>]  编辑步骤
  step remove <rpa_id> <index>   删除步骤（index 从 0 开始）
  step move <rpa_id> <from> <to> 移动步骤位置
  step types                     查看所有可用步骤类型

执行控制:
  run <rpa_id> --device <device_id> [--mode single|loop]  执行 RPA
  stop --device <device_id>      停止执行
  status --device <device_id>    查看执行状态
  history [--device <device_id>] [--limit <n>]  查看执行历史

步骤类型:
  change_os_and_wait   改机重启        set_proxy_and_wait  设置代理
  set_location         设置定位        install_app_and_wait 安装应用
  get_root             应用提权        network_check       网络检测
  download_url         URL下载         download_cloud      云端下载
  http_request         HTTP请求        shell               Shell命令
  start_bot            启动脚本        run_script          执行脚本
  execute_repo_script  仓库脚本        set_variables       设置变量
  condition_check      条件判断

示例:
  jpy-cloud rpa list
  jpy-cloud rpa create --name "自动化测试"
  jpy-cloud rpa step add 1 --type shell --name "清理缓存" --params '{"command":"rm -rf /data/cache/*"}'
  jpy-cloud rpa step edit 1 0 --set command="ls -la"
  jpy-cloud rpa step types
  jpy-cloud rpa run 1 --device 12345678
  jpy-cloud rpa status --device 12345678
  jpy-cloud rpa history --limit 10
`)
}

// ========== HTTP 客户端 ==========

func rpaGet(path string, result interface{}) error {
	url := LocalServerURL + "/api/rpa" + path
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("请求失败（本地服务是否已启动？）: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiResp struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}
	if apiResp.Error != "" {
		return fmt.Errorf("API 错误: %s", apiResp.Error)
	}
	if result != nil && len(apiResp.Data) > 0 {
		return json.Unmarshal(apiResp.Data, result)
	}
	return nil
}

func rpaPost(path string, data interface{}, result interface{}) error {
	url := LocalServerURL + "/api/rpa" + path
	jsonData, _ := json.Marshal(data)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("请求失败（本地服务是否已启动？）: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiResp struct {
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}
	if apiResp.Error != "" {
		return fmt.Errorf("API 错误: %s", apiResp.Error)
	}
	if result != nil && len(apiResp.Data) > 0 {
		return json.Unmarshal(apiResp.Data, result)
	}
	return nil
}

func rpaPut(path string, data interface{}, result interface{}) error {
	url := LocalServerURL + "/api/rpa" + path
	jsonData, _ := json.Marshal(data)
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败（本地服务是否已启动？）: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiResp struct {
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}
	if apiResp.Error != "" {
		return fmt.Errorf("API 错误: %s", apiResp.Error)
	}
	if result != nil && len(apiResp.Data) > 0 {
		return json.Unmarshal(apiResp.Data, result)
	}
	return nil
}

func rpaDeleteRequest(path string) error {
	url := LocalServerURL + "/api/rpa" + path
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("DELETE", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败（本地服务是否已启动？）: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}
	if apiResp.Error != "" {
		return fmt.Errorf("API 错误: %s", apiResp.Error)
	}
	return nil
}

// ========== 流程管理 ==========

func rpaList(args []string) {
	jsonOutput := containsFlag(args, "--json")

	var flows []RpaFlow
	if err := rpaGet("/flows", &flows); err != nil {
		fmt.Printf("获取 RPA 列表失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(flows)
		return
	}

	if len(flows) == 0 {
		fmt.Println("暂无 RPA 流程")
		return
	}

	fmt.Printf("%-6s %-20s %-8s %-10s %s\n", "ID", "名称", "步骤数", "状态", "更新时间")
	fmt.Println(strings.Repeat("-", 70))
	for _, f := range flows {
		status := "启用"
		if !f.Enabled {
			status = "禁用"
		}
		fmt.Printf("%-6d %-20s %-8d %-10s %s\n",
			f.ID,
			truncateString(f.Name, 18),
			len(f.Steps),
			status,
			f.UpdatedAt.Format("2006-01-02 15:04"),
		)
	}
}

func rpaShow(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud rpa show <id>")
		return
	}

	id := args[0]
	jsonOutput := containsFlag(args, "--json")

	var flow RpaFlow
	if err := rpaGet("/flows/"+id, &flow); err != nil {
		fmt.Printf("获取 RPA 失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(flow)
		return
	}

	fmt.Printf("ID: %d\n", flow.ID)
	fmt.Printf("名称: %s\n", flow.Name)
	fmt.Printf("描述: %s\n", flow.Description)
	fmt.Printf("状态: %s\n", boolToStatus(flow.Enabled))
	fmt.Printf("创建时间: %s\n", flow.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("更新时间: %s\n", flow.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("\n步骤列表 (%d 个):\n", len(flow.Steps))
	fmt.Println(strings.Repeat("-", 60))

	for i, step := range flow.Steps {
		paramsJSON, _ := json.Marshal(step.Params)
		fmt.Printf("[%d] %s (%s)\n", i, step.Name, step.Type)
		fmt.Printf("    参数: %s\n", string(paramsJSON))
	}
}

func rpaCreate(args []string) {
	name := getFlagValue(args, "--name")
	desc := getFlagValue(args, "--desc")

	if name == "" {
		fmt.Println("用法: jpy-cloud rpa create --name <名称> [--desc <描述>]")
		return
	}

	reqData := map[string]interface{}{
		"name":        name,
		"description": desc,
		"steps":       []interface{}{},
		"enabled":     true,
	}

	var flow RpaFlow
	if err := rpaPost("/flows", reqData, &flow); err != nil {
		fmt.Printf("创建 RPA 失败: %v\n", err)
		return
	}

	fmt.Printf("创建成功，ID: %d\n", flow.ID)
}

func rpaDeleteCmd(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud rpa delete <id>")
		return
	}

	id := args[0]

	// 先获取详情确认存在
	var flow RpaFlow
	if err := rpaGet("/flows/"+id, &flow); err != nil {
		fmt.Printf("获取 RPA 失败: %v\n", err)
		return
	}

	if err := rpaDeleteRequest("/flows/" + id); err != nil {
		fmt.Printf("删除 RPA 失败: %v\n", err)
		return
	}

	fmt.Printf("已删除 RPA: %s (ID: %s)\n", flow.Name, id)
}

// ========== 步骤管理 ==========

func rpaStep(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud rpa step <list|add|remove|move> ...")
		return
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "list", "ls":
		rpaStepList(subArgs)
	case "add":
		rpaStepAdd(subArgs)
	case "edit", "set":
		rpaStepEdit(subArgs)
	case "remove", "rm":
		rpaStepRemove(subArgs)
	case "move", "mv":
		rpaStepMove(subArgs)
	case "types":
		rpaStepTypes(subArgs)
	default:
		fmt.Printf("未知命令: rpa step %s\n", subCmd)
	}
}

func rpaStepList(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud rpa step list <rpa_id>")
		return
	}

	id := args[0]
	jsonOutput := containsFlag(args, "--json")

	var flow RpaFlow
	if err := rpaGet("/flows/"+id, &flow); err != nil {
		fmt.Printf("获取 RPA 失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(flow.Steps)
		return
	}

	fmt.Printf("RPA: %s (ID: %d)\n", flow.Name, flow.ID)
	fmt.Printf("步骤数: %d\n\n", len(flow.Steps))

	if len(flow.Steps) == 0 {
		fmt.Println("暂无步骤")
		return
	}

	fmt.Printf("%-6s %-20s %-15s %s\n", "序号", "名称", "类型", "参数")
	fmt.Println(strings.Repeat("-", 80))
	for i, step := range flow.Steps {
		paramsJSON, _ := json.Marshal(step.Params)
		paramsStr := truncateString(string(paramsJSON), 40)
		fmt.Printf("%-6d %-20s %-15s %s\n", i, truncateString(step.Name, 18), step.Type, paramsStr)
	}
}

func rpaStepAdd(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud rpa step add <rpa_id> --type <类型> --name <名称> [--params <JSON>]")
		return
	}

	rpaID := args[0]
	stepType := getFlagValue(args, "--type")
	stepName := getFlagValue(args, "--name")
	paramsStr := getFlagValue(args, "--params")

	if stepType == "" || stepName == "" {
		fmt.Println("用法: jpy-cloud rpa step add <rpa_id> --type <类型> --name <名称> [--params <JSON>]")
		return
	}

	// 获取当前流程
	var flow RpaFlow
	if err := rpaGet("/flows/"+rpaID, &flow); err != nil {
		fmt.Printf("获取 RPA 失败: %v\n", err)
		return
	}

	// 解析参数
	var params map[string]interface{}
	if paramsStr != "" {
		if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
			fmt.Printf("参数 JSON 格式错误: %v\n", err)
			return
		}
	} else {
		params = make(map[string]interface{})
	}

	// 生成步骤 ID
	stepID := fmt.Sprintf("step_%s_%d", rpaID, time.Now().UnixNano())

	newStep := RpaStep{
		ID:     stepID,
		Name:   stepName,
		Type:   stepType,
		Params: params,
	}

	flow.Steps = append(flow.Steps, newStep)

	// 更新流程
	if err := rpaPut("/flows/"+rpaID, flow, nil); err != nil {
		fmt.Printf("保存失败: %v\n", err)
		return
	}

	fmt.Printf("已添加步骤: %s (类型: %s, 序号: %d)\n", stepName, stepType, len(flow.Steps)-1)
}

func rpaStepRemove(args []string) {
	if len(args) < 2 {
		fmt.Println("用法: jpy-cloud rpa step remove <rpa_id> <index>")
		return
	}

	rpaID := args[0]
	index, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Printf("无效的序号: %s\n", args[1])
		return
	}

	// 获取当前流程
	var flow RpaFlow
	if err := rpaGet("/flows/"+rpaID, &flow); err != nil {
		fmt.Printf("获取 RPA 失败: %v\n", err)
		return
	}

	if index < 0 || index >= len(flow.Steps) {
		fmt.Printf("序号超出范围: %d (共 %d 个步骤)\n", index, len(flow.Steps))
		return
	}

	removedStep := flow.Steps[index]
	flow.Steps = append(flow.Steps[:index], flow.Steps[index+1:]...)

	// 更新流程
	if err := rpaPut("/flows/"+rpaID, flow, nil); err != nil {
		fmt.Printf("保存失败: %v\n", err)
		return
	}

	fmt.Printf("已删除步骤: %s (序号: %d)\n", removedStep.Name, index)
}

func rpaStepEdit(args []string) {
	if len(args) < 2 {
		fmt.Println(`用法: jpy-cloud rpa step edit <rpa_id> <index> [选项]

选项:
  --name <名称>         修改步骤名称
  --type <类型>         修改步骤类型
  --params <JSON>       替换整个参数（JSON 格式）
  --set <key=value>     修改单个参数（可多次使用）
  --unset <key>         删除单个参数

示例:
  jpy-cloud rpa step edit 1 0 --name "新名称"
  jpy-cloud rpa step edit 1 2 --set targetUrl=ws://192.168.1.100:1003/ws/device
  jpy-cloud rpa step edit 1 2 --set maxRetries=30 --set retryInterval=5
  jpy-cloud rpa step edit 1 0 --params '{"command":"ls -la","outputVar":"result"}'
  jpy-cloud rpa step edit 1 3 --unset deviceName`)
		return
	}

	rpaID := args[0]
	index, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Printf("无效的序号: %s\n", args[1])
		return
	}

	// 获取当前流程
	var flow RpaFlow
	if err := rpaGet("/flows/"+rpaID, &flow); err != nil {
		fmt.Printf("获取 RPA 失败: %v\n", err)
		return
	}

	if index < 0 || index >= len(flow.Steps) {
		fmt.Printf("序号超出范围: %d (共 %d 个步骤)\n", index, len(flow.Steps))
		return
	}

	step := &flow.Steps[index]
	changed := false

	// --name
	if name := getFlagValue(args[2:], "--name"); name != "" {
		step.Name = name
		changed = true
	}

	// --type
	if stepType := getFlagValue(args[2:], "--type"); stepType != "" {
		step.Type = stepType
		changed = true
	}

	// --params（替换整个参数）
	if paramsStr := getFlagValue(args[2:], "--params"); paramsStr != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
			fmt.Printf("参数 JSON 格式错误: %v\n", err)
			return
		}
		step.Params = params
		changed = true
	}

	// --set key=value（可多次出现）
	for i := 2; i < len(args); i++ {
		if args[i] == "--set" && i+1 < len(args) {
			kv := args[i+1]
			eqIdx := strings.Index(kv, "=")
			if eqIdx <= 0 {
				fmt.Printf("无效的 --set 格式: %s（应为 key=value）\n", kv)
				return
			}
			key := kv[:eqIdx]
			value := kv[eqIdx+1:]

			// 尝试解析为数字或布尔
			if step.Params == nil {
				step.Params = make(map[string]interface{})
			}
			step.Params[key] = parseValue(value)
			changed = true
			i++ // 跳过 value
		}
	}

	// --unset key（可多次出现）
	for i := 2; i < len(args); i++ {
		if args[i] == "--unset" && i+1 < len(args) {
			key := args[i+1]
			delete(step.Params, key)
			changed = true
			i++
		}
	}

	if !changed {
		fmt.Println("未指定任何修改项，使用 --name/--type/--params/--set/--unset")
		return
	}

	// 更新流程
	if err := rpaPut("/flows/"+rpaID, flow, nil); err != nil {
		fmt.Printf("保存失败: %v\n", err)
		return
	}

	paramsJSON, _ := json.Marshal(step.Params)
	fmt.Printf("已更新步骤 [%d] %s (%s)\n", index, step.Name, step.Type)
	fmt.Printf("  参数: %s\n", string(paramsJSON))
}

// parseValue 尝试将字符串解析为合适的类型
func parseValue(s string) interface{} {
	// 布尔
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	// 整数
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	// 浮点数
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// JSON 对象/数组
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		var v interface{}
		if json.Unmarshal([]byte(s), &v) == nil {
			return v
		}
	}
	// 字符串
	return s
}

// StepTypeInfo 步骤类型信息
type StepTypeInfo struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	SubSteps []string `json:"subSteps"`
}

func rpaStepTypes(args []string) {
	jsonOutput := containsFlag(args, "--json")

	var types []StepTypeInfo
	if err := rpaGet("/step-types", &types); err != nil {
		fmt.Printf("获取步骤类型失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(types)
		return
	}

	if len(types) == 0 {
		fmt.Println("暂无步骤类型")
		return
	}

	fmt.Printf("%-25s %-15s %s\n", "类型标识", "名称", "子步骤")
	fmt.Println(strings.Repeat("-", 70))
	for _, t := range types {
		subSteps := "-"
		if len(t.SubSteps) > 0 {
			subSteps = strings.Join(t.SubSteps, " → ")
		}
		fmt.Printf("%-25s %-15s %s\n", t.Type, t.Name, subSteps)
	}
}

func rpaStepMove(args []string) {
	if len(args) < 3 {
		fmt.Println("用法: jpy-cloud rpa step move <rpa_id> <from> <to>")
		return
	}

	rpaID := args[0]
	from, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Printf("无效的源位置: %s\n", args[1])
		return
	}
	to, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Printf("无效的目标位置: %s\n", args[2])
		return
	}

	// 获取当前流程
	var flow RpaFlow
	if err := rpaGet("/flows/"+rpaID, &flow); err != nil {
		fmt.Printf("获取 RPA 失败: %v\n", err)
		return
	}

	if from < 0 || from >= len(flow.Steps) || to < 0 || to >= len(flow.Steps) {
		fmt.Printf("位置超出范围 (共 %d 个步骤)\n", len(flow.Steps))
		return
	}

	// 移动步骤
	step := flow.Steps[from]
	flow.Steps = append(flow.Steps[:from], flow.Steps[from+1:]...)
	newSteps := make([]RpaStep, 0, len(flow.Steps)+1)
	newSteps = append(newSteps, flow.Steps[:to]...)
	newSteps = append(newSteps, step)
	newSteps = append(newSteps, flow.Steps[to:]...)
	flow.Steps = newSteps

	// 更新流程
	if err := rpaPut("/flows/"+rpaID, flow, nil); err != nil {
		fmt.Printf("保存失败: %v\n", err)
		return
	}

	fmt.Printf("已移动步骤: %s (%d -> %d)\n", step.Name, from, to)
}

// ========== 执行控制 ==========

func rpaRun(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: jpy-cloud rpa run <rpa_id> --device <device_id> [--mode single|loop]")
		return
	}

	rpaID := args[0]
	deviceIDStr := getFlagValue(args, "--device")
	if deviceIDStr == "" {
		fmt.Println("缺少参数: --device <device_id>")
		return
	}

	mode := getFlagValue(args, "--mode")
	if mode == "" {
		mode = "single"
	}

	// 检查 RPA 是否存在
	var flow RpaFlow
	if err := rpaGet("/flows/"+rpaID, &flow); err != nil {
		fmt.Printf("获取 RPA 失败: %v\n", err)
		return
	}

	// 绑定设备 RPA
	bindData := map[string]interface{}{
		"rpaId": flow.ID,
		"mode":  mode,
	}
	if err := rpaPost("/devices/"+deviceIDStr+"/bind", bindData, nil); err != nil {
		fmt.Printf("绑定设备 RPA 失败: %v\n", err)
		return
	}

	// 启动执行
	if err := rpaPost("/devices/"+deviceIDStr+"/start", nil, nil); err != nil {
		fmt.Printf("启动执行失败: %v\n", err)
		return
	}

	fmt.Printf("已启动 RPA 执行\n")
	fmt.Printf("  RPA: %s (ID: %d)\n", flow.Name, flow.ID)
	fmt.Printf("  设备: %s\n", deviceIDStr)
	fmt.Printf("  模式: %s\n", mode)
	fmt.Println("\n使用 'jpy-cloud rpa status --device", deviceIDStr, "' 查看执行状态")
}

func rpaStop(args []string) {
	deviceIDStr := getFlagValue(args, "--device")
	if deviceIDStr == "" {
		fmt.Println("用法: jpy-cloud rpa stop --device <device_id>")
		return
	}

	if err := rpaPost("/devices/"+deviceIDStr+"/stop", nil, nil); err != nil {
		fmt.Printf("停止执行失败: %v\n", err)
		return
	}

	fmt.Printf("已停止设备 %s 的 RPA 执行\n", deviceIDStr)
}

func rpaStatus(args []string) {
	deviceIDStr := getFlagValue(args, "--device")
	if deviceIDStr == "" {
		fmt.Println("用法: jpy-cloud rpa status --device <device_id>")
		return
	}

	jsonOutput := containsFlag(args, "--json")

	var config DeviceConfig
	if err := rpaGet("/devices/"+deviceIDStr, &config); err != nil {
		fmt.Printf("获取设备状态失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(config)
		return
	}

	fmt.Printf("设备 ID: %d\n", config.DeviceID)
	fmt.Printf("RPA ID: %d\n", config.RpaID)
	if config.RpaName != "" {
		fmt.Printf("RPA 名称: %s\n", config.RpaName)
	}
	fmt.Printf("状态: %s\n", config.Status)
	if config.Mode != "" {
		fmt.Printf("模式: %s\n", config.Mode)
	}

	// 当前步骤显示优化
	if config.TotalSteps > 0 {
		stepInfo := fmt.Sprintf("%d/%d", config.CurrentStep, config.TotalSteps)
		if config.StepName != "" {
			stepInfo += " - " + config.StepName
		}
		if config.SubStep > 0 || config.SubStepName != "" {
			subInfo := fmt.Sprintf(" (子步骤: %d", config.SubStep)
			if config.SubStepName != "" {
				subInfo += " - " + config.SubStepName
			}
			subInfo += ")"
			stepInfo += subInfo
		}
		fmt.Printf("当前步骤: %s\n", stepInfo)
	} else {
		fmt.Printf("当前步骤: %d (子步骤: %d)\n", config.CurrentStep, config.SubStep)
	}

	fmt.Printf("循环次数: %d\n", config.LoopCount)
	fmt.Printf("成功/失败: %d/%d\n", config.SuccessCount, config.FailCount)

	// 计算已耗时
	elapsed := config.TotalTime
	if config.Status == "running" && config.LoopStartAt != nil {
		elapsed = int(time.Since(*config.LoopStartAt).Seconds())
	}
	fmt.Printf("已耗时: %d 秒\n", elapsed)

	// 脚本状态（如果有）
	if config.ScriptStatus != "" {
		fmt.Printf("脚本状态: %s (%d%%)\n", config.ScriptStatus, config.ScriptProgress)
	}

	// 最后错误（始终显示，即使为空也显示"无"）
	if config.LastError != "" {
		fmt.Printf("最后错误: %s\n", config.LastError)
	}

	if config.StartedAt != nil {
		fmt.Printf("开始时间: %s\n", config.StartedAt.Format("2006-01-02 15:04:05"))
	} else if config.LoopStartAt != nil {
		fmt.Printf("开始时间: %s\n", config.LoopStartAt.Format("2006-01-02 15:04:05"))
	}
}

func rpaHistory(args []string) {
	deviceIDStr := getFlagValue(args, "--device")
	limitStr := getFlagValue(args, "--limit")
	jsonOutput := containsFlag(args, "--json")

	limit := "20"
	if limitStr != "" {
		limit = limitStr
	}

	var histories []ExecutionHistory
	var err error

	if deviceIDStr != "" {
		err = rpaGet("/devices/"+deviceIDStr+"/history?limit="+limit, &histories)
	} else {
		err = rpaGet("/history?limit="+limit, &histories)
	}

	if err != nil {
		fmt.Printf("获取执行历史失败: %v\n", err)
		return
	}

	if jsonOutput {
		OutputJSON(histories)
		return
	}

	if len(histories) == 0 {
		fmt.Println("暂无执行历史")
		return
	}

	fmt.Printf("%-6s %-10s %-20s %-10s %-8s %-10s %s\n", "ID", "设备ID", "RPA名称", "状态", "进度", "耗时", "开始时间")
	fmt.Println(strings.Repeat("-", 90))
	for _, h := range histories {
		progress := fmt.Sprintf("%d/%d", h.CurrentStep, h.TotalSteps)
		duration := fmt.Sprintf("%ds", h.Duration)
		fmt.Printf("%-6d %-10d %-20s %-10s %-8s %-10s %s\n",
			h.ID,
			h.DeviceID,
			truncateString(h.RpaName, 18),
			h.Status,
			progress,
			duration,
			h.StartedAt.Format("01-02 15:04"),
		)
	}
}

// ========== 辅助函数 ==========

func containsFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func getFlagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func boolToStatus(b bool) string {
	if b {
		return "启用"
	}
	return "禁用"
}
