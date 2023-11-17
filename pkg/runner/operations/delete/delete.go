package delete

import (
	"context"

	"github.com/kyverno/chainsaw/pkg/client"
	"github.com/kyverno/chainsaw/pkg/runner/logging"
	"github.com/kyverno/chainsaw/pkg/runner/namespacer"
	"github.com/kyverno/chainsaw/pkg/runner/operations"
	"github.com/kyverno/chainsaw/pkg/runner/operations/internal"
	"github.com/kyverno/kyverno/ext/output/color"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type operation struct {
	client     client.Client
	obj        ctrlclient.Object
	namespacer namespacer.Namespacer
}

func New(client client.Client, obj ctrlclient.Object, namespacer namespacer.Namespacer) operations.Operation {
	return &operation{
		client:     client,
		obj:        obj,
		namespacer: namespacer,
	}
}

func (o *operation) Exec(ctx context.Context) (_err error) {
	logger := logging.FromContext(ctx).WithResource(o.obj)
	defer func() {
		if _err == nil {
			logger.Log(logging.Delete, logging.DoneStatus, color.BoldGreen)
		} else {
			logger.Log(logging.Delete, logging.ErrorStatus, color.BoldRed, logging.ErrSection(_err))
		}
	}()
	if o.namespacer != nil {
		if err := o.namespacer.Apply(o.obj); err != nil {
			return err
		}
	}
	logger.Log(logging.Delete, logging.RunStatus, color.BoldFgCyan)
	candidates, _err := internal.Read(ctx, o.obj, o.client)
	if _err != nil {
		if errors.IsNotFound(_err) {
			return nil
		}
		return _err
	}
	for i := range candidates {
		err := o.client.Delete(ctx, &candidates[i])
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	gvk := o.obj.GetObjectKind().GroupVersionKind()
	for i := range candidates {
		if err := wait.PollUntilContextCancel(ctx, internal.PollInterval, true, func(ctx context.Context) (bool, error) {
			var actual unstructured.Unstructured
			actual.SetGroupVersionKind(gvk)
			err := o.client.Get(ctx, client.ObjectKey(&candidates[i]), &actual)
			if err != nil {
				if errors.IsNotFound(err) {
					return true, nil
				}
				return false, err
			}
			return false, nil
		}); err != nil {
			return err
		}
	}
	return nil
}