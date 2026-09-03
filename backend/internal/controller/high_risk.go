package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
	"regexp"

	"github.com/gin-gonic/gin"
)

// ListHighRisk 高危指令规则列表
func ListHighRisk(c *gin.Context) {
	var list []model.HighRiskCommand
	database.DB.Order("id").Find(&list)
	utils.Success(c, list)
}

// HighRiskReq 高危规则请求
type HighRiskReq struct {
	Name      string `json:"name" binding:"required"`
	Pattern   string `json:"pattern" binding:"required"`
	MatchType string `json:"matchType"`
	RiskLevel string `json:"riskLevel"`
	Action    string `json:"action"`
	Enabled   int    `json:"enabled"`
	Remark    string `json:"remark"`
}

// CreateHighRisk 新建规则
func CreateHighRisk(c *gin.Context) {
	var req HighRiskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	h := model.HighRiskCommand{
		Name: req.Name, Pattern: req.Pattern, MatchType: req.MatchType,
		RiskLevel: req.RiskLevel, Action: req.Action, Enabled: req.Enabled, Remark: req.Remark,
	}
	database.DB.Create(&h)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateHighRisk 更新规则
func UpdateHighRisk(c *gin.Context) {
	id := c.Param("id")
	var req HighRiskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var h model.HighRiskCommand
	if err := database.DB.First(&h, id).Error; err != nil {
		utils.Fail(c, "规则不存在")
		return
	}
	h.Name = req.Name
	h.Pattern = req.Pattern
	h.MatchType = req.MatchType
	h.RiskLevel = req.RiskLevel
	h.Action = req.Action
	h.Enabled = req.Enabled
	h.Remark = req.Remark
	database.DB.Save(&h)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteHighRisk 删除规则（内置不可删）
func DeleteHighRisk(c *gin.Context) {
	id := c.Param("id")
	var h model.HighRiskCommand
	if err := database.DB.First(&h, id).Error; err != nil {
		utils.Fail(c, "规则不存在")
		return
	}
	if h.Builtin == 1 {
		utils.Fail(c, "内置规则不可删除")
		return
	}
	database.DB.Delete(&h)
	utils.SuccessMsg(c, "删除成功")
}

// RiskHit 命中结果
type RiskHit struct {
	Blocked bool
	Rule    *model.HighRiskCommand
}

// CheckCommand 校验命令是否命中高危规则（供 Web Terminal 调用）
func CheckCommand(cmd string) RiskHit {
	var rules []model.HighRiskCommand
	database.DB.Where("enabled = ?", 1).Find(&rules)
	for i := range rules {
		r := &rules[i]
		hit := false
		switch r.MatchType {
		case "exact":
			hit = cmd == r.Pattern
		case "prefix":
			hit = len(cmd) >= len(r.Pattern) && cmd[:len(r.Pattern)] == r.Pattern
		case "regex":
			if re, err := regexp.Compile(r.Pattern); err == nil {
				hit = re.MatchString(cmd)
			}
		default:
			hit = cmd == r.Pattern
		}
		if hit {
			return RiskHit{Blocked: r.Action == "block", Rule: r}
		}
	}
	return RiskHit{Blocked: false, Rule: nil}
}
