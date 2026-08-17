package syncExt

/**
 * @BelongProject yunka
 * @BelongPackage syncExt
 * @Description:
 *
 * @Copyright 2021 - Powered By 云咖
 * @Author: fworld
 * @Date:  2021/4/12 上午12:11
 * @Version V1.0
 */

func Multi(errs ...func() error) error {
	//if len(errs) == 0 {
	//	return nil
	//}
	//ch := make(chan error, len(errs))
	//defer close(ch)
	//for _, h := range errs {
	//	go func() {
	//		select {
	//		default:
	//			ch <- h()
	//		}
	//	}()
	//}
	//
	//for err := range ch {
	//	if err != nil {
	//		return err
	//	}
	//}

	return nil
}
