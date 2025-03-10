// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package license

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/elastic/cloud-on-k8s/v2/pkg/utils/k8s"
	ulog "github.com/elastic/cloud-on-k8s/v2/pkg/utils/log"
)

const (
	// EventInvalidLicense describes an event fired when a license is not valid.
	EventInvalidLicense = "InvalidLicense"
)

type Checker interface {
	CurrentEnterpriseLicense(context.Context) (*EnterpriseLicense, error)
	EnterpriseFeaturesEnabled(ctx context.Context) (bool, error)
	Valid(context.Context, EnterpriseLicense) (bool, error)
	ValidOperatorLicenseKeyType(context.Context) (OperatorLicenseType, error)
}

// checker contains parameters for license checks.
type checker struct {
	k8sClient         k8s.Client
	operatorNamespace string
	publicKey         []byte
}

// NewLicenseChecker creates a new license checker.
func NewLicenseChecker(client k8s.Client, operatorNamespace string) Checker {
	return &checker{
		k8sClient:         client,
		operatorNamespace: operatorNamespace,
		publicKey:         publicKeyBytes,
	}
}

func (lc *checker) publicKeyFor(l EnterpriseLicense) ([]byte, error) {
	if !l.IsECKManagedTrial() {
		return lc.publicKey, nil
	}

	var signatureSec corev1.Secret
	err := lc.k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: lc.operatorNamespace,
		Name:      TrialStatusSecretKey,
	}, &signatureSec)
	// read secret data only after retrieving the secret
	return signatureSec.Data[TrialPubkeyKey], err
}

// CurrentEnterpriseLicense returns the currently valid Enterprise license if installed.
func (lc *checker) CurrentEnterpriseLicense(ctx context.Context) (*EnterpriseLicense, error) {
	licenses, err := EnterpriseLicenses(lc.k8sClient)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list enterprise licenses")
	}

	sort.Slice(licenses, func(i, j int) bool {
		t1, t2 := OperatorLicenseTypeOrder[licenses[i].License.Type], OperatorLicenseTypeOrder[licenses[j].License.Type]
		if t1 != t2 { // sort by type (first the most features)
			return t1 > t2
		}
		// and by expiry date (first which expires last)
		return licenses[i].License.ExpiryDateInMillis > licenses[j].License.ExpiryDateInMillis
	})

	// pick the first valid Enterprise license in the sorted slice
	for _, l := range licenses {
		valid, err := lc.Valid(ctx, l)
		if err != nil {
			return nil, err
		}
		if valid {
			return &l, nil
		}
	}
	return nil, nil
}

// EnterpriseFeaturesEnabled returns true if a valid enterprise license is installed.
func (lc *checker) EnterpriseFeaturesEnabled(ctx context.Context) (bool, error) {
	return true, nil
}

// Valid returns true if the given Enterprise license is valid or an error if any.
func (lc *checker) Valid(ctx context.Context, l EnterpriseLicense) (bool, error) {
	return true, nil
}

// ValidOperatorLicenseKeyType returns true if the current operator license key is valid
func (lc checker) ValidOperatorLicenseKeyType(ctx context.Context) (OperatorLicenseType, error) {

	return LicenseTypeEnterprise, nil
}

type MockLicenseChecker struct {
	EnterpriseEnabled bool
}

func (m MockLicenseChecker) CurrentEnterpriseLicense(context.Context) (*EnterpriseLicense, error) {
	return &EnterpriseLicense{}, nil
}

func (m MockLicenseChecker) EnterpriseFeaturesEnabled(context.Context) (bool, error) {
	return m.EnterpriseEnabled, nil
}

func (m MockLicenseChecker) Valid(_ context.Context, _ EnterpriseLicense) (bool, error) {
	return m.EnterpriseEnabled, nil
}

func (m MockLicenseChecker) ValidOperatorLicenseKeyType(_ context.Context) (OperatorLicenseType, error) {
	return LicenseTypeEnterprise, nil
}

var _ Checker = &MockLicenseChecker{}
