package apperror

func NewVectorCollectionAlreadyExists(collectionName string) *AppError {
	return NewServiceMsg("向量集合已存在，禁止重复创建：" + collectionName)
}
