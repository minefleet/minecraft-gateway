package edge

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type DomainSnapshot = map[string]types.NamespacedName

func StartADS(ctx context.Context, snapshot <-chan DomainSnapshot) {

}
