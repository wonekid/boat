package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Result 统一响应
type Result struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// PageData 分页数据
type PageData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

const (
	CodeSuccess = 0
	CodeError   = 1
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Result{Code: CodeSuccess, Msg: "success", Data: data})
}

func SuccessMsg(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Result{Code: CodeSuccess, Msg: msg, Data: nil})
}

func Page(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Result{Code: CodeSuccess, Msg: "success", Data: PageData{
		List: list, Total: total, Page: page, PageSize: pageSize,
	}})
}

func Fail(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Result{Code: CodeError, Msg: msg, Data: nil})
}

func FailCode(c *gin.Context, httpStatus int, msg string) {
	c.JSON(httpStatus, Result{Code: CodeError, Msg: msg, Data: nil})
}

func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, Result{Code: CodeError, Msg: msg, Data: nil})
}

func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, Result{Code: CodeError, Msg: msg, Data: nil})
}
