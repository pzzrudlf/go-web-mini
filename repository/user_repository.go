package repository

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"go-lim/common"
	"go-lim/model"
	"go-lim/util"
	"go-lim/vo"
	"strings"
	"time"
)

type IUserRepository interface {
	Login(user *model.User) (*model.User, error)                   // 登录
	GetCurrentUser(c *gin.Context) model.User                      // 获取当前登录用户信息
	GetUserById(id uint) (model.User, error)                       // 获取单个用户
	GetUsers(req *vo.UserListRequest) ([]model.User, int64, error) // 获取用户列表
	ChangePwd(username string, newPasswd string) error             // 修改密码
	CreateUser(user *model.User) error                             // 创建用户
	UpdateUserById(id uint, user *model.User) error                // 更新用户
	BatchDeleteUserByIds(ids []string) error
}

type UserRepository struct {
}

// 当前用户信息缓存，避免频繁查询数据库
var userInfoCache = cache.New(24*time.Hour, 48*time.Hour)

// UserRepository构造函数
func NewUserRepository() IUserRepository {
	return UserRepository{}
}

// 登录
func (ur UserRepository) Login(user *model.User) (*model.User, error) {
	// 根据用户名查询用户(正常状态:用户状态正常)
	var firstUser model.User
	err := common.DB.
		Where("username = ?", user.Username).
		Preload("Roles").
		First(&firstUser).Error
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	// 判断用户的状态
	userStatus := firstUser.Status
	if userStatus != 1 {
		return nil, errors.New("用户被禁用")
	}

	// 判断用户拥有的所有角色的状态,全部角色都被禁用则不能登录
	roles := firstUser.Roles
	isValidate := false
	for _, role := range roles {
		// 有一个正常状态的角色就可以登录
		if role.Status == 1 {
			isValidate = true
			break
		}
	}

	if !isValidate {
		return nil, errors.New("用户角色被禁用")
	}

	// 校验密码
	err = util.ComparePasswd(firstUser.Password, user.Password)
	if err != nil {
		return &firstUser, errors.New("密码错误")
	}
	return &firstUser, nil
}

// 获取当前登录用户信息
func (ur UserRepository) GetCurrentUser(c *gin.Context) model.User {
	var newUser model.User
	ctxUser, exist := c.Get("user")
	if !exist {
		return newUser
	}
	u, _ := ctxUser.(model.User)

	// 先查询缓存
	cacheUser, found := userInfoCache.Get(u.Username)
	var user model.User
	if found {
		user = cacheUser.(model.User)
	} else {
		// 缓存中没有就查询数据库
		user, _ = ur.GetUserById(u.ID)
	}
	return user
}

// 获取单个用户(正常状态)
// 需要缓存，减少数据库访问
func (ur UserRepository) GetUserById(id uint) (model.User, error) {
	fmt.Println("GetUserById---查数据库")
	var user model.User
	err := common.DB.Where("id = ?", id).
		Where("status = ?", 1).
		Preload("Roles").First(&user).Error

	// 缓存
	userInfoCache.Set(user.Username, user, cache.DefaultExpiration)

	return user, err
}

// 获取用户列表
func (ur UserRepository) GetUsers(req *vo.UserListRequest) ([]model.User, int64, error) {
	var list []model.User
	db := common.DB.Model(&model.User{}).Order("created_at DESC")

	username := strings.TrimSpace(req.Username)
	if username != "" {
		db = db.Where("username LIKE ?", fmt.Sprintf("%%%s%%", username))
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname != "" {
		db = db.Where("nickname LIKE ?", fmt.Sprintf("%%%s%%", nickname))
	}
	mobile := strings.TrimSpace(req.Mobile)
	if mobile != "" {
		db = db.Where("mobile LIKE ?", fmt.Sprintf("%%%s%%", mobile))
	}
	status := req.Status
	if status != 0 {
		db = db.Where("status = ?", status)
	}
	// 当pageNum > 0 且 pageSize > 0 才分页
	//记录总条数
	var total int64
	err := db.Count(&total).Error
	if err != nil {
		return list, total, err
	}
	pageNum := int(req.PageNum)
	pageSize := int(req.PageSize)
	if pageNum > 0 && pageSize > 0 {
		err = db.Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	} else {
		err = db.Find(&list).Error
	}
	return list, total, err
}

// 修改密码
func (ur UserRepository) ChangePwd(username string, hashNewPasswd string) error {
	err := common.DB.Model(&model.User{}).Where("username = ?", username).Update("password", hashNewPasswd).Error
	// 如果修改密码成功，则更新当前用户信息缓存
	// 先查询缓存
	cacheUser, found := userInfoCache.Get(username)
	if err == nil {
		if found {
			user := cacheUser.(model.User)
			user.Password = hashNewPasswd
			userInfoCache.Set(username, user, cache.DefaultExpiration)
		} else {
			// 没有缓存就查询用户信息缓存
			var user model.User
			common.DB.Where("username = ?", username).First(&user)
			userInfoCache.Set(username, user, cache.DefaultExpiration)
		}
	}

	return err
}

// 创建用户
func (ur UserRepository) CreateUser(user *model.User) error {
	err := common.DB.Create(user).Error
	return err
}

func (ur UserRepository) UpdateUserById(id uint, user *model.User) error {
	err := common.DB.Model(&model.User{}).Where("id = ?", id).Updates(user).Error
	return err
}

func (ur UserRepository) BatchDeleteUserByIds(ids []string) error {
	panic("implement me")
}
