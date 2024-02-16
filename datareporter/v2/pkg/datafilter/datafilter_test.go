// Copyright 2024 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datafilter_test

import (
	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-marketplace/redhat-marketplace-operator/datareporter/v2/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("DataFilter", func() {
	var (
		drc         v1alpha1.DataReporterConfig
		metadataMap map[string]string
	)

	BeforeEach(func() {

		metadataMap = make(map[string]string)
		metadataMap["k"] = "v"

		drc = v1alpha1.DataReporterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "datareporterconfig",
				Namespace: "default",
			},
			Spec: v1alpha1.DataReporterConfigSpec{
				UserConfigs: []v1alpha1.UserConfig{
					v1alpha1.UserConfig{
						UserName: "testuser",
						Metadata: metadataMap,
					},
				},
				DataFilters: []v1alpha1.DataFilter{
					v1alpha1.DataFilter{
						Selector: v1alpha1.Selector{
							MatchExpressions: []string{
								`$.properties.productId`,
								`$[?($.properties.source != null)]`,
								`$[?($.properties.unit == "AppPoints")]`,
								`$[?($.properties.quantity >= 0)]`,
								`$[?($.timestamp != null)]`,
							},
							MatchUsers: []string{"testuser"},
						},
						ManifestType: "dataReporter",
						Transformer: v1alpha1.Transformer{
							TransformerType: "kazaam",
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "kazaam-configmap",
								},
								Key: "kazaam.json",
							},
						},
						AltDestinations: []v1alpha1.Destination{
							v1alpha1.Destination{
								Transformer: v1alpha1.Transformer{
									TransformerType: "kazaam",
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kazaam-configmap",
										},
										Key: "kazaam.json",
									},
								},
								URL:           "https://test/api",
								URLSuffixExpr: "$.subscriptionId",
								Header: v1alpha1.Header{
									Secret: corev1.LocalObjectReference{
										Name: "dest-header-map-secret",
									},
								},
								Authorization: v1alpha1.Authorization{
									URL: "https://test/auth",
									Header: v1alpha1.Header{
										Secret: corev1.LocalObjectReference{
											Name: "auth-header-map-secret",
										},
									},
									AuthDestHeader:       "Authorization",
									AuthDestHeaderPrefix: "Bearer ",
									TokenExpr:            "$.token",
									BodyData: v1alpha1.SecretKeyRef{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "auth-data-secret",
											},
											Key: "auth",
										},
									},
								},
							},
						},
					},
				},
				TLSConfig: &v1alpha1.TLSConfig{
					InsecureSkipVerify: false,
					CACerts: []corev1.SecretKeySelector{
						corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "tls-config-secret",
							},
							Key: "ca.crt",
						},
					},
					Certificates: []v1alpha1.Certificate{
						v1alpha1.Certificate{
							ClientCert: v1alpha1.SecretKeyRef{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-config-secret",
									},
									Key: "tls.crt",
								},
							},
							ClientKey: v1alpha1.SecretKeyRef{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-config-secret",
									},
									Key: "tls.key",
								},
							},
						},
					},
					CipherSuites: []string{"TLS_AES_128_GCM_SHA256",
						"TLS_AES_256_GCM_SHA384",
						"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
						"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
						"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
					MinVersion: "VersionTLS12",
				},
				ConfirmDelivery: ptr.Bool(false),
			},
		}

	})

	Describe("Building DataFilters", func() {
		Context("with valid DataReporterConfig.DataFilters", func() {
			It("should build DataFilters", func() {
				drcGood := drc
				err := dataFilters.Build(&drcGood)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid DataReporterConfig.DataFilters", func() {
			It("should error on bad MatchExpression", func() {
				drcBadMatchExpr := drc
				drcBadMatchExpr.Spec.DataFilters[0].Selector.MatchExpressions = append(drcBadMatchExpr.Spec.DataFilters[0].Selector.MatchExpressions, `[?(`)

				err := dataFilters.Build(&drcBadMatchExpr)
				Expect(err).To(HaveOccurred())
			})

			// TODO: implement transformer
			/*

				It("should error on Transformer ConfigMap Name not found", func() {
					drcBadTransformerCM := drc
					drcBadTransformerCM.Spec.DataFilters[0].Transformer.ConfigMapKeyRef.Name = "name-not-here"

					err := dataFilters.Build(&drcBadTransformerCM)
					Expect(err).To(HaveOccurred())
				})

				It("should error on Transformer ConfigMap Key not found", func() {
					drcBadTransformerKey := drc
					drcBadTransformerKey.Spec.DataFilters[0].Transformer.ConfigMapKeyRef.Key = "key-not-here"

					err := dataFilters.Build(&drcBadTransformerKey)
					Expect(err).To(HaveOccurred())
				})

				It("should error on kazaam Transformer configuration malformed", func() {
					drcBadTransformerConfig := drc
					drcBadTransformerConfig.Spec.DataFilters[0].Transformer.ConfigMapKeyRef.Name = "kazaam-configmap-bad"

					err := dataFilters.Build(&drcBadTransformerConfig)
					Expect(err).To(HaveOccurred())
				})


					It("should error on Destination Transformer ConfigMap Name not found", func() {
						drcBadDestTransformerCM := drc
						drcBadDestTransformerCM.Spec.DataFilters[0].AltDestinations[0].Transformer.ConfigMapKeyRef.Name = "name-not-here"

						err := dataFilters.Build(&drcBadDestTransformerCM)
						Expect(err).To(HaveOccurred())
					})

					It("should error on Destination Transformer ConfigMap Key not found", func() {
						drcBadDestTransformerKey := drc
						drcBadDestTransformerKey.Spec.DataFilters[0].AltDestinations[0].Transformer.ConfigMapKeyRef.Key = "key-not-here"

						err := dataFilters.Build(&drcBadDestTransformerKey)
						Expect(err).To(HaveOccurred())
					})

					It("should error on kazaam Destination Transformer configuration malformed", func() {
						drcBadDestTransformerConfig := drc
						drcBadDestTransformerConfig.Spec.DataFilters[0].AltDestinations[0].Transformer.ConfigMapKeyRef.Name = "kazaam-configmap-bad"

						err := dataFilters.Build(&drcBadDestTransformerConfig)
						Expect(err).To(HaveOccurred())
					})
			*/

			It("should error on Destination URL malformed", func() {
				drcBadDestURL := drc
				drcBadDestURL.Spec.DataFilters[0].AltDestinations[0].URL = ":https:/test/api"

				err := dataFilters.Build(&drcBadDestURL)
				Expect(err).To(HaveOccurred())
			})

			It("should error on Destination URLSuffixExpr malformed", func() {
				drcBadDestURLSuffixExpr := drc
				drcBadDestURLSuffixExpr.Spec.DataFilters[0].AltDestinations[0].URLSuffixExpr = `[?(`

				err := dataFilters.Build(&drcBadDestURLSuffixExpr)
				Expect(err).To(HaveOccurred())
			})

			It("should error on Destination HeaderSecret malformed", func() {
				drcBadDestHeaderSecret := drc
				drcBadDestHeaderSecret.Spec.DataFilters[0].AltDestinations[0].Header.Secret.Name = "dest-header-secret-not-here"

				err := dataFilters.Build(&drcBadDestHeaderSecret)
				Expect(err).To(HaveOccurred())
			})

			It("should error on Authorization URL malformed", func() {
				drcBadAuthURL := drc
				drcBadAuthURL.Spec.DataFilters[0].AltDestinations[0].Authorization.URL = ":https:/test/auth"

				err := dataFilters.Build(&drcBadAuthURL)
				Expect(err).To(HaveOccurred())
			})

			It("should error on Authorization HeaderSecret malformed", func() {
				drcBadAuthHeaderSecret := drc
				drcBadAuthHeaderSecret.Spec.DataFilters[0].AltDestinations[0].Authorization.Header.Secret.Name = "auth-header-secret-not-here"

				err := dataFilters.Build(&drcBadAuthHeaderSecret)
				Expect(err).To(HaveOccurred())
			})

			It("should error on Authorization TokenExpr malformed", func() {
				drcBadAuthTokenExpr := drc
				drcBadAuthTokenExpr.Spec.DataFilters[0].AltDestinations[0].Authorization.TokenExpr = `[?(`

				err := dataFilters.Build(&drcBadAuthTokenExpr)
				Expect(err).To(HaveOccurred())
			})

			// TODO: implement payload
			/*
				It("should error on Authorization Data Secret Name not found", func() {
					drcBadAuthDataSecretName := drc
					drcBadAuthDataSecretName.Spec.DataFilters[0].AltDestinations[0].Authorization.DataSecret.Name = "auth-data-secret-name-not-here"

					err := dataFilters.Build(&drcBadAuthDataSecretName)
					Expect(err).To(HaveOccurred())
				})

				It("should error on Authorization Data Secret Key not found", func() {
					drcBadAuthDataSecretKey := drc
					drcBadAuthDataSecretKey.Spec.DataFilters[0].AltDestinations[0].Authorization.DataSecret.Key = "auth-data-secret-key-not-here"

					err := dataFilters.Build(&drcBadAuthDataSecretKey)
					Expect(err).To(HaveOccurred())
				})
			*/

			It("should error on TLSConfig secret not found", func() {
				drcBadTLSConfigSecretName := drc
				drcBadTLSConfigSecretName.Spec.TLSConfig.CACerts[0].Name = "no-secret-here"

				err := dataFilters.Build(&drcBadTLSConfigSecretName)
				Expect(err).To(HaveOccurred())
			})

			It("should error on TLSConfig bad secret data", func() {
				drcBadTLSConfigKeyData := drc
				drcBadTLSConfigKeyData.Spec.TLSConfig.CACerts[0].Key = "bad.crt"

				err := dataFilters.Build(&drcBadTLSConfigKeyData)
				Expect(err).To(HaveOccurred())
			})

			It("should error on TLSConfig bad MinVersion", func() {
				drcBadTLSConfigMinVersion := drc
				drcBadTLSConfigMinVersion.Spec.TLSConfig.MinVersion = "not-a-version"

				err := dataFilters.Build(&drcBadTLSConfigMinVersion)
				Expect(err).To(HaveOccurred())
			})

			It("should error on TLSConfig bad CipherSuites", func() {
				drcBadTLSConfigCipherSuites := drc
				drcBadTLSConfigCipherSuites.Spec.TLSConfig.CipherSuites = []string{"badcipher1", "badcipher2"}

				err := dataFilters.Build(&drcBadTLSConfigCipherSuites)
				Expect(err).To(HaveOccurred())
			})

		})

	})

})
