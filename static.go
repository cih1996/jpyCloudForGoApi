package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

// GetStaticFS 返回嵌入的静态文件系统（去掉 static 前缀）
func GetStaticFS() http.FileSystem {
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(subFS)
}

// GetStaticSubFS 返回嵌入的 fs.FS（用于读取文件内容）
func GetStaticSubFS() fs.FS {
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return subFS
}
