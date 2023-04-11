package core

/**
 * @BelongProject yunka
 * @BelongPackage core
 * @Description:
 *
 * @Copyright 2021 - Powered By 云咖
 * @Author: fworld
 * @Date:  2021/4/11 下午4:57
 * @Version V1.0
 */
type Repository interface {
	GetRepo() interface{}
	SetRepo(interface{})
}

type BaseRepository struct {
	repo interface{}
}

func (repo *BaseRepository) GetRepo() interface{} {
	return repo.repo
}

func (repo *BaseRepository) SetRepo(r interface{}) {
	repo.repo = r
}
