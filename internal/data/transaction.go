package data

import (
	"Orion_Live/internal/repository"

	"gorm.io/gorm"
)

// UnitOfWork 定义了我们事务管理器的接口
type UnitOfWork interface {
	// Execute 将一个函数包裹在数据库事务中执行。
	// 它会为这个函数提供能在事务中工作的 Repositories。
	Execute(func(repos *TransactionalRepositories) error) error
}

// TransactionalRepositories 持有所有需要在同一个事务中操作的 Repository。
type TransactionalRepositories struct {
	VideoRepo   repository.VideoRepository
	CommentRepo repository.CommentRepository
	// 如果需要，未来可以加入 UserRepo, LikeRepo 等
}

// db是事务的入口和管理者
type gormUnitOfWork struct {
	db          *gorm.DB
	videoRepo   repository.VideoRepository
	commentRepo repository.CommentRepository
}

// NewUnitOfWork 创建一个新的、基于GORM的“工作单元”。
// 注意，它接收的是原始的、非事务的 repositories。
func NewUnitOfWork(db *gorm.DB, videoRepo repository.VideoRepository, commentRepo repository.CommentRepository) UnitOfWork {
	return &gormUnitOfWork{
		db:          db,
		videoRepo:   videoRepo,
		commentRepo: commentRepo,
	}
}

// 契约：fn func(repos *TransactionalRepositories) error
// 只能接收长这样的函数，并为其创建事务；将符合契约的Repositories作为参数，“注入”到业务逻辑函数中
func (u *gormUnitOfWork) Execute(fn func(repos *TransactionalRepositories) error) error {
	// GORM创建了一个事务，并把这个事务的句柄作为参数tx传递给了这个匿名函数
	return u.db.Transaction(func(tx *gorm.DB) error {
		// 临时创建“一次性”的、绑定了特定事务的Repo副本
		transactionalRepos := &TransactionalRepositories{
			VideoRepo:   u.videoRepo.WithTx(tx),
			CommentRepo: u.commentRepo.WithTx(tx),
		}
		// 回调结构（Callback），回头去调用最初调用者托付给它的具体业务逻辑，并将其执行结果作为整个事务成功或失败的依据
		return fn(transactionalRepos)
	})
}
