package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOrUpdateService(c client.Client, service *corev1.Service) error {
	var newer corev1.Service
	if err := c.Get(context.TODO(), client.ObjectKeyFromObject(service), &newer); err != nil {
		if apierrors.IsNotFound(err) {
			return c.Create(context.TODO(), service)
		}

		return err
	}

	service.ResourceVersion = newer.ResourceVersion
	return c.Update(context.TODO(), service)
}
