package provider

import "github.com/baihua19941101/cdnManage/internal/model"

type Type string

const (
	TypeUnknown      Type = Type(model.ProviderTypeUnknown)
	TypeAliyun       Type = "aliyun"
	TypeTencentCloud Type = "tencent_cloud"
	TypeHuaweiCloud  Type = "huawei_cloud"
	TypeQiniu        Type = "qiniu"
)

func (t Type) String() string {
	return string(t)
}

func IsKnownType(t Type) bool {
	switch t {
	case TypeUnknown, TypeAliyun, TypeTencentCloud, TypeHuaweiCloud, TypeQiniu:
		return true
	default:
		return false
	}
}

func SupportedTypes() []Type {
	return []Type{
		TypeAliyun,
		TypeTencentCloud,
		TypeHuaweiCloud,
		TypeQiniu,
	}
}
