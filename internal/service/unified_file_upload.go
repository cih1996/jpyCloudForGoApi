package service

import (
	"fmt"
	"io"
	"net/http"
	"port-mapping-demo/pkg/logger"

	"adminApi/tbFileCtl"

	"github.com/gin-gonic/gin"
)

// HandleFastUpload 秒传检测：检查文件hash是否已存在
func HandleFastUpload(c *gin.Context) {
	if err := ensureGlobalApi(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "未登录或连接断开: " + err.Error()})
		return
	}

	var req struct {
		Hash     string `json:"hash" binding:"required"`
		FileName string `json:"fileName" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "参数错误: " + err.Error()})
		return
	}

	logger.LogInfo("[FileUpload] FastUpload check: hash=%s, fileName=%s", req.Hash, req.FileName)

	hash := req.Hash
	fileName := req.FileName
	exists, errPkg := GetGlobalApi().TbFileCtl.FastUpload(tbFileCtl.FastUploadReq{
		Hash:     &hash,
		FileName: &fileName,
	})
	if errPkg != nil {
		logger.LogError("[FileUpload] FastUpload failed: %s", errPkg.Msg)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "秒传检测失败: " + errPkg.Msg})
		return
	}

	logger.LogInfo("[FileUpload] FastUpload result: exists=%v", exists)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"exists": exists}})
}

// HandleFileUpload 文件上传代理：接收文件 → 获取COS预签名URL → 上传到COS → 注册文件
func HandleFileUpload(c *gin.Context) {
	if err := ensureGlobalApi(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "未登录或连接断开: " + err.Error()})
		return
	}

	// 1. 接收 multipart 文件
	hash := c.PostForm("hash")
	fileName := c.PostForm("fileName")
	if hash == "" || fileName == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "缺少 hash 或 fileName 参数"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "文件读取失败: " + err.Error()})
		return
	}
	defer file.Close()

	logger.LogInfo("[FileUpload] Upload start: hash=%s, fileName=%s", hash, fileName)

	// 2. 获取 COS 预签名上传 URL
	cosUrl, errPkg := GetGlobalApi().TbFileCtl.GetUploadUrl(tbFileCtl.GetUploadUrlReq{
		Hash:     &hash,
		FileName: &fileName,
	})
	if errPkg != nil {
		logger.LogError("[FileUpload] GetUploadUrl failed: %s", errPkg.Msg)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "获取上传地址失败: " + errPkg.Msg})
		return
	}

	logger.LogInfo("[FileUpload] Got COS upload URL, uploading...")

	// 3. 流式上传到 COS（PUT 请求）
	putReq, err := http.NewRequest("PUT", cosUrl, file)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "构建上传请求失败: " + err.Error()})
		return
	}
	putReq.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{}
	resp, err := client.Do(putReq)
	if err != nil {
		logger.LogError("[FileUpload] COS PUT failed: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "上传到存储服务失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("COS上传返回异常 status=%d, body=%s", resp.StatusCode, string(body))
		logger.LogError("[FileUpload] %s", errMsg)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": errMsg})
		return
	}

	logger.LogInfo("[FileUpload] COS upload success, registering file...")

	// 4. 注册文件到集控平台
	errPkg = GetGlobalApi().TbFileCtl.AddCosFile(tbFileCtl.AddCosFileReq{
		Hash:     &hash,
		FileName: &fileName,
	})
	if errPkg != nil {
		logger.LogError("[FileUpload] AddCosFile failed: %s", errPkg.Msg)
		c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "文件注册失败: " + errPkg.Msg})
		return
	}

	logger.LogInfo("[FileUpload] Upload complete: %s", fileName)
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "上传成功", "data": gin.H{
		"hash":     hash,
		"fileName": fileName,
	}})
}
