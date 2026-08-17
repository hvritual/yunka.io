package trie

/**
* @Description: TODO
* @date 2019-04-23
* @version V1.0
 */
type Trier interface {
	Get(key string) interface{}
	Put(key string, value interface{}) bool
	Delete(key string) bool
	Walk(walker WalkFunc) error
}
