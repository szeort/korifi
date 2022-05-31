package services

import (
	"context"
	"errors"
	"fmt"

	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"code.cloudfoundry.org/korifi/controllers/webhooks"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	ServiceBindingEntityType = "servicebinding"

	ServiceBindingErrorType             = "ServiceBindingValidationError"
	DuplicateServiceBindingErrorType    = "DuplicateServiceBindingError"
	duplicateServiceBindingErrorMessage = "Service binding already exists: App: %s Service Instance: %s"
)

// log is for logging in this package.
var cfservicebindinglog = logf.Log.WithName("cfservicebinding-validator")

func (v *CFServiceBindingValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&korifiv1alpha1.CFServiceBinding{}).
		WithValidator(v).
		Complete()
}

type CFServiceBindingValidator struct {
	duplicateValidator NameValidator
}

func NewCFServiceBindingValidator(duplicateValidator NameValidator) *CFServiceBindingValidator {
	return &CFServiceBindingValidator{
		duplicateValidator: duplicateValidator,
	}
}

//+kubebuilder:webhook:path=/validate-korifi-cloudfoundry-org-v1alpha1-cfservicebinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=korifi.cloudfoundry.org,resources=cfservicebindings,verbs=create;update;delete,versions=v1alpha1,name=vcfservicebinding.korifi.cloudfoundry.org,admissionReviewVersions={v1,v1beta1}

var _ webhook.CustomValidator = &CFServiceBindingValidator{}

func (v *CFServiceBindingValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	serviceBinding, ok := obj.(*korifiv1alpha1.CFServiceBinding)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a CFServiceBinding but got a %T", obj))
	}

	lockName := generateServiceBindingLock(serviceBinding)

	validationErr := v.duplicateValidator.ValidateCreate(ctx, cfservicebindinglog, serviceBinding.Namespace, lockName)

	if validationErr != nil {
		if errors.Is(validationErr, webhooks.ErrorDuplicateName) {
			errorMessage := fmt.Sprintf(duplicateServiceBindingErrorMessage, serviceBinding.Spec.AppRef.Name, serviceBinding.Spec.Service.Name)
			return errors.New(webhooks.ValidationError{Type: DuplicateServiceBindingErrorType, Message: errorMessage}.Marshal())
		}

		return errors.New(webhooks.AdmissionUnknownErrorReason())
	}

	return nil
}

func (v *CFServiceBindingValidator) ValidateUpdate(ctx context.Context, oldObj, updatedObj runtime.Object) error {
	oldServiceBinding, ok := oldObj.(*korifiv1alpha1.CFServiceBinding)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a CFServiceBinding but got a %T", oldObj))
	}
	updatedServiceBinding, ok := updatedObj.(*korifiv1alpha1.CFServiceBinding)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a CFServiceBinding but got a %T", updatedObj))
	}

	if oldServiceBinding.Spec.AppRef.Name != updatedServiceBinding.Spec.AppRef.Name {
		return webhooks.ValidationError{Type: ServiceBindingErrorType, Message: "AppRef.Name is immutable"}
	}
	if oldServiceBinding.Spec.Service.Name != updatedServiceBinding.Spec.Service.Name {
		return webhooks.ValidationError{Type: ServiceBindingErrorType, Message: "Service.Name is immutable"}
	}
	if oldServiceBinding.Spec.Service.Namespace != updatedServiceBinding.Spec.Service.Namespace {
		return webhooks.ValidationError{Type: ServiceBindingErrorType, Message: "Service.Namespace is immutable"}
	}

	return nil
}

func (v *CFServiceBindingValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	serviceBinding, ok := obj.(*korifiv1alpha1.CFServiceBinding)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a CFServiceBinding but got a %T", obj))
	}

	lockName := generateServiceBindingLock(serviceBinding)

	validationErr := v.duplicateValidator.ValidateDelete(ctx, cfservicebindinglog, serviceBinding.Namespace, lockName)

	if validationErr != nil {
		return errors.New(webhooks.AdmissionUnknownErrorReason())
	}

	return nil
}

func generateServiceBindingLock(serviceBinding *korifiv1alpha1.CFServiceBinding) string {
	return fmt.Sprintf("sb::%s::%s::%s", serviceBinding.Spec.AppRef.Name, serviceBinding.Spec.Service.Namespace, serviceBinding.Spec.Service.Name)
}
