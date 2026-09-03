package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
	"io"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// ListScripts 脚本库
func ListScripts(c *gin.Context) {
	keyword := c.Query("keyword")
	q := database.DB.Model(&model.Script{})
	if keyword != "" {
		q = q.Where("name LIKE ?", "%"+keyword+"%")
	}
	var list []model.Script
	q.Order("id DESC").Find(&list)
	utils.Success(c, list)
}

// ScriptReq 脚本请求
type ScriptReq struct {
	Name    string `json:"name" binding:"required"`
	Lang    string `json:"lang"`
	Content string `json:"content"`
	Remark  string `json:"remark"`
}

// CreateScript 新建脚本
func CreateScript(c *gin.Context) {
	var req ScriptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	s := model.Script{Name: req.Name, Lang: req.Lang, Content: req.Content, Remark: req.Remark}
	database.DB.Create(&s)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateScript 更新脚本
func UpdateScript(c *gin.Context) {
	id := c.Param("id")
	var req ScriptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var s model.Script
	if err := database.DB.First(&s, id).Error; err != nil {
		utils.Fail(c, "脚本不存在")
		return
	}
	s.Name = req.Name
	s.Lang = req.Lang
	s.Content = req.Content
	s.Remark = req.Remark
	database.DB.Save(&s)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteScript 删除脚本
func DeleteScript(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&model.Script{}, id)
	utils.SuccessMsg(c, "删除成功")
}

// GetScript 脚本详情
func GetScript(c *gin.Context) {
	id := c.Param("id")
	var s model.Script
	if err := database.DB.First(&s, id).Error; err != nil {
		utils.Fail(c, "脚本不存在")
		return
	}
	utils.Success(c, s)
}

// UploadScript 上传脚本文件（.sh/.py/.bash 等），内容写入脚本库
// 表单字段：file(必填)、name/remark/lang(可选，缺省时由文件名推断)
func UploadScript(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.Fail(c, "请选择要上传的脚本文件")
		return
	}
	f, err := file.Open()
	if err != nil {
		utils.Fail(c, "文件打开失败")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		utils.Fail(c, "读取文件失败")
		return
	}
	if len(raw) == 0 {
		utils.Fail(c, "文件内容为空")
		return
	}
	if len(raw) > 1<<20 { // 1MB 上限
		utils.Fail(c, "脚本文件过大（上限 1MB）")
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
	}
	lang := strings.TrimSpace(c.PostForm("lang"))
	if lang == "" {
		switch strings.ToLower(filepath.Ext(file.Filename)) {
		case ".py":
			lang = "python"
		default:
			lang = "shell"
		}
	}
	s := model.Script{
		Name:    name,
		Lang:    lang,
		Content: string(raw),
		Remark:  strings.TrimSpace(c.PostForm("remark")),
	}
	database.DB.Create(&s)
	utils.Success(c, gin.H{"id": s.ID, "name": s.Name, "lang": s.Lang})
}
