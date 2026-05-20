package apperror

// NewVectorCollectionAlreadyExists 构造向量集合重复创建的服务端错误。
func NewVectorCollectionAlreadyExists(collectionName string) *AppError {
	return NewServiceMsg("向量集合已存在，禁止重复创建：" + collectionName)
}
