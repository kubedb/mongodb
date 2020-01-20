/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package e2e_test

import (
	"fmt"
	"os"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"kubedb.dev/mongodb/test/e2e/framework"
	"kubedb.dev/mongodb/test/e2e/matcher"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
	store "kmodules.xyz/objectstore-api/api/v1"
	ofst "kmodules.xyz/offshoot-api/api/v1"
	stashV1alpha1 "stash.appscode.dev/stash/apis/stash/v1alpha1"
	stashV1beta1 "stash.appscode.dev/stash/apis/stash/v1beta1"
)

const (
	S3_BUCKET_NAME             = "S3_BUCKET_NAME"
	GCS_BUCKET_NAME            = "GCS_BUCKET_NAME"
	AZURE_CONTAINER_NAME       = "AZURE_CONTAINER_NAME"
	SWIFT_CONTAINER_NAME       = "SWIFT_CONTAINER_NAME"
	MONGO_INITDB_ROOT_USERNAME = "MONGO_INITDB_ROOT_USERNAME"
	MONGO_INITDB_ROOT_PASSWORD = "MONGO_INITDB_ROOT_PASSWORD"
	MONGO_INITDB_DATABASE      = "MONGO_INITDB_DATABASE"
)

var _ = Describe("MongoDB", func() {
	var (
		err            error
		f              *framework.Invocation
		mongodb        *api.MongoDB
		garbageMongoDB *api.MongoDBList
		snapshotPVC    *core.PersistentVolumeClaim
		secret         *core.Secret
		skipMessage    string
		verifySharding bool
		enableSharding bool
		dbName         string
	)

	BeforeEach(func() {
		f = root.Invoke()
		mongodb = f.MongoDBStandalone()
		garbageMongoDB = new(api.MongoDBList)
		secret = nil
		skipMessage = ""
		verifySharding = false
		enableSharding = false
		dbName = "kubedb"
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over MongoDB objects")
		root.CleanMongoDB()
		By("Delete left over workloads if exists any")
		root.CleanWorkloadLeftOvers()

		if snapshotPVC != nil {
			err := f.DeletePersistentVolumeClaim(snapshotPVC.ObjectMeta)
			if err != nil && !kerr.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			f.PrintDebugHelpers()
		}
	})

	var createAndWaitForRunning = func() {
		By("Create MongoDB: " + mongodb.Name)
		err = f.CreateMongoDB(mongodb)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running mongodb")
		f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

		By("Wait for AppBinding to create")
		f.EventuallyAppBinding(mongodb.ObjectMeta).Should(BeTrue())

		By("Check valid AppBinding Specs")
		err := f.CheckAppBindingSpec(mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Ping mongodb database")
		f.EventuallyPingMongo(mongodb.ObjectMeta)
	}

	var deleteTestResource = func() {
		if mongodb == nil {
			Skip("Skipping")
		}

		By("Check if mongodb " + mongodb.Name + " exists.")
		mg, err := f.GetMongoDB(mongodb.ObjectMeta)
		if err != nil && kerr.IsNotFound(err) {
			// MongoDB was not created. Hence, rest of cleanup is not necessary.
			return
		}
		Expect(err).NotTo(HaveOccurred())

		By("Update mongodb to set spec.terminationPolicy = WipeOut")
		_, err = f.PatchMongoDB(mg.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb")
		err = f.DeleteMongoDB(mongodb.ObjectMeta)
		if err != nil && kerr.IsNotFound(err) {
			// MongoDB was not created. Hence, rest of cleanup is not necessary.
			return
		}
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb to be deleted")
		f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

		By("Wait for mongodb resources to be wipedOut")
		f.EventuallyWipedOut(mongodb.ObjectMeta).Should(Succeed())
	}

	Describe("Test", func() {

		BeforeEach(func() {
			if f.StorageClass == "" {
				Skip("Missing StorageClassName. Provide as flag to test this.")
			}
		})

		AfterEach(func() {
			// Delete test resource
			deleteTestResource()

			for _, mg := range garbageMongoDB.Items {
				*mongodb = mg
				// Delete test resource
				deleteTestResource()
			}

			if secret != nil {
				err := f.DeleteSecret(secret.ObjectMeta)
				if !kerr.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		Context("General", func() {

			Context("With PVC - Halt", func() {

				var shouldRunWithPVC = func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}
					// Create MongoDB
					createAndWaitForRunning()

					if enableSharding {
						By("Enable sharding for db:" + dbName)
						f.EventuallyEnableSharding(mongodb.ObjectMeta, dbName).Should(BeTrue())
					}
					if verifySharding {
						By("Check if db " + dbName + " is set to partitioned")
						f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
					}

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

					By("Halt MongoDB: Update mongodb to set spec.halted = true")
					_, err := f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.Halted = true
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for halted/paused mongodb")
					f.EventuallyMongoDBPhase(mongodb.ObjectMeta).Should(Equal(api.DatabasePhasePaused))

					By("Resume MongoDB: Update mongodb to set spec.halted = false")
					_, err = f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.Halted = false
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Ping mongodb database")
					f.EventuallyPingMongo(mongodb.ObjectMeta)

					if verifySharding {
						By("Check if db " + dbName + " is set to partitioned")
						f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
					}

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())
				}

				It("should run successfully", shouldRunWithPVC)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Replicas = types.Int32P(3)
					})
					It("should run successfully", shouldRunWithPVC)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						verifySharding = true
						mongodb = f.MongoDBShard()
					})

					Context("-", func() {
						BeforeEach(func() {
							enableSharding = false
						})
						It("should run successfully", shouldRunWithPVC)
					})

					Context("With Sharding Enabled database", func() {
						BeforeEach(func() {
							enableSharding = true
						})
						It("should run successfully", shouldRunWithPVC)
					})
				})

			})

			Context("PDB", func() {
				It("should run evictions on MongoDB successfully", func() {
					mongodb = f.MongoDBRS()
					mongodb.Spec.Replicas = types.Int32P(3)
					// Create MongoDB
					createAndWaitForRunning()
					//Evict a MongoDB pod
					By("Try to evict pods")
					err = f.EvictPodsFromStatefulSet(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should run evictions on Sharded MongoDB successfully", func() {
					mongodb = f.MongoDBShard()
					mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					mongodb.Spec.ShardTopology.Shard.Shards = int32(1)
					mongodb.Spec.ShardTopology.ConfigServer.Replicas = int32(3)
					mongodb.Spec.ShardTopology.Mongos.Replicas = int32(3)
					mongodb.Spec.ShardTopology.Shard.Replicas = int32(3)
					// Create MongoDB
					createAndWaitForRunning()
					//Evict a MongoDB pod from each sts and deploy
					By("Try to evict pods from each statefulset")
					err := f.EvictPodsFromStatefulSet(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
					By("Try to evict pods from deployment")
					err = f.EvictPodsFromDeployment(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})
			})
			Context("with custom SA Name", func() {
				BeforeEach(func() {
					customSecret := f.SecretForDatabaseAuthentication(mongodb.ObjectMeta, false)
					mongodb.Spec.DatabaseSecret = &core.SecretVolumeSource{
						SecretName: customSecret.Name,
					}
					err = f.CreateSecret(customSecret)
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyHalt
				})

				It("should start and resume without shard successfully", func() {
					//shouldTakeSnapshot()
					Expect(err).NotTo(HaveOccurred())
					mongodb.Spec.PodTemplate = &ofst.PodTemplateSpec{
						Spec: ofst.PodSpec{
							ServiceAccountName: "my-custom-sa",
						},
					}
					createAndWaitForRunning()
					if mongodb == nil {
						Skip("Skipping")
					}

					By("Check if MongoDB " + mongodb.Name + " exists.")
					_, err = f.GetMongoDB(mongodb.ObjectMeta)
					if err != nil {
						if kerr.IsNotFound(err) {
							// MongoDB was not created. Hence, rest of cleanup is not necessary.
							return
						}
						Expect(err).NotTo(HaveOccurred())
					}

					By("Delete mongodb: " + mongodb.Name)
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					if err != nil {
						if kerr.IsNotFound(err) {
							// MongoDB was not created. Hence, rest of cleanup is not necessary.
							log.Infof("Skipping rest of cleanup. Reason: MongoDB %s is not found.", mongodb.Name)
							return
						}
						Expect(err).NotTo(HaveOccurred())
					}

					By("Wait for mongodb to be deleted")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

					By("Resume DB")
					createAndWaitForRunning()
				})

				It("should start and resume with shard successfully", func() {
					mongodb = f.MongoDBShard()
					//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					mongodb.Spec.ShardTopology.Shard.Shards = int32(1)
					mongodb.Spec.ShardTopology.Shard.MongoDBNode.Replicas = int32(1)
					mongodb.Spec.ShardTopology.ConfigServer.MongoDBNode.Replicas = int32(1)
					mongodb.Spec.ShardTopology.Mongos.MongoDBNode.Replicas = int32(1)

					mongodb.Spec.ShardTopology.ConfigServer.PodTemplate = ofst.PodTemplateSpec{
						Spec: ofst.PodSpec{
							ServiceAccountName: "my-custom-sa-configserver",
						},
					}
					mongodb.Spec.ShardTopology.Mongos.PodTemplate = ofst.PodTemplateSpec{
						Spec: ofst.PodSpec{
							ServiceAccountName: "my-custom-sa-mongos",
						},
					}
					mongodb.Spec.ShardTopology.Shard.PodTemplate = ofst.PodTemplateSpec{
						Spec: ofst.PodSpec{
							ServiceAccountName: "my-custom-sa-shard",
						},
					}

					createAndWaitForRunning()
					if mongodb == nil {
						Skip("Skipping")
					}

					By("Check if MongoDB " + mongodb.Name + " exists.")
					_, err := f.GetMongoDB(mongodb.ObjectMeta)
					if err != nil {
						if kerr.IsNotFound(err) {
							// MongoDB was not created. Hence, rest of cleanup is not necessary.
							return
						}
						Expect(err).NotTo(HaveOccurred())
					}

					By("Delete mongodb: " + mongodb.Name)
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					if err != nil {
						if kerr.IsNotFound(err) {
							// MongoDB was not created. Hence, rest of cleanup is not necessary.
							log.Infof("Skipping rest of cleanup. Reason: MongoDB %s is not found.", mongodb.Name)
							return
						}
						Expect(err).NotTo(HaveOccurred())
					}

					By("Wait for mongodb to be deleted")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

					By("Resume DB")
					createAndWaitForRunning()
				})
			})
		})

		Context("Initialize", func() {

			Context("With Script", func() {
				var configMap *core.ConfigMap

				BeforeEach(func() {
					configMap = f.ConfigMapForInitialization()
					err := f.CreateConfigMap(configMap)
					Expect(err).NotTo(HaveOccurred())

					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					}
				})

				AfterEach(func() {
					err := f.DeleteConfigMap(configMap.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should run successfully", func() {
					// Create MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Replicas = types.Int32P(3)
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
					})
					It("should Initialize successfully", func() {
						// Create MongoDB
						createAndWaitForRunning()

						By("Checking Inserted Document")
						f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
					})
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
					})
					It("should Initialize successfully", func() {
						// Create MongoDB
						createAndWaitForRunning()

						By("Checking Inserted Document")
						f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
					})
				})

			})

			// To run this test,
			// 1st: Deploy stash latest operator
			// 2nd: create mongodb related tasks and functions from
			// `kubedb.dev/mongodb/hack/dev/examples/stash01_config.yaml`
			// TODO: may be moved to another file?
			Context("With Stash", func() {
				var bc *stashV1beta1.BackupConfiguration
				var rs *stashV1beta1.RestoreSession
				var repo *stashV1alpha1.Repository

				var anotherMongoDB *api.MongoDB

				BeforeEach(func() {
					if !f.FoundStashCRDs() {
						Skip("Skipping tests for stash integration. reason: stash operator is not running.")
					}
					anotherMongoDB = f.MongoDBStandalone()
				})

				AfterEach(func() {
					By("Deleting RestoreSession")
					err = f.DeleteRestoreSession(rs.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Deleting Repository")
					err = f.DeleteRepository(repo.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				var createAndWaitForInitializing = func() {
					By("Creating MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Initializing mongodb")
					f.EventuallyMongoDBPhase(mongodb.ObjectMeta).Should(Equal(api.DatabasePhaseInitializing))

					By("Wait for AppBinding to create")
					f.EventuallyAppBinding(mongodb.ObjectMeta).Should(BeTrue())

					By("Check valid AppBinding Specs")
					err = f.CheckAppBindingSpec(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Ping mongodb database")
					f.EventuallyPingMongo(mongodb.ObjectMeta)
				}

				var shouldInitializeFromStash = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					if enableSharding {
						By("Enable sharding for db:" + dbName)
						f.EventuallyEnableSharding(mongodb.ObjectMeta, dbName).Should(BeTrue())
					}
					if verifySharding {
						By("Check if db " + dbName + " is set to partitioned")
						f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
					}

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 25).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 25).Should(BeTrue())

					By("Create Secret")
					err = f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					By("Create Repositories")
					err = f.CreateRepository(repo)
					Expect(err).NotTo(HaveOccurred())

					By("Create BackupConfiguration")
					err = f.CreateBackupConfiguration(bc)
					Expect(err).NotTo(HaveOccurred())

					By("Check for snapshot count in stash-repository")
					f.EventuallySnapshotInRepository(repo.ObjectMeta).Should(matcher.MoreThan(2))

					By("Delete BackupConfiguration to stop backup scheduling")
					err = f.DeleteBackupConfiguration(bc.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					oldMongoDB, err := f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					garbageMongoDB.Items = append(garbageMongoDB.Items, *oldMongoDB)

					By("Create mongodb from stash")
					mongodb = anotherMongoDB // without value?
					rs = f.RestoreSession(mongodb.ObjectMeta, repo)
					mongodb.Spec.DatabaseSecret = oldMongoDB.Spec.DatabaseSecret
					mongodb.Spec.Init = &api.InitSpec{
						StashRestoreSession: &core.LocalObjectReference{
							Name: rs.Name,
						},
					}

					// Create and wait for running MongoDB
					createAndWaitForInitializing()

					By("Create Stash-RestoreSession")
					err = f.CreateRestoreSession(rs)
					Expect(err).NotTo(HaveOccurred())

					// eventually backupsession succeeded
					By("Check for Succeeded restoreSession")
					f.EventuallyRestoreSessionPhase(rs.ObjectMeta).Should(Equal(stashV1beta1.RestoreSessionSucceeded))

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					if verifySharding && mongodb.Spec.ShardTopology != nil {
						By("Check if db " + dbName + " is set to partitioned")
						f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
					}

					By("Checking previously Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 25).Should(BeTrue())
				}

				Context("From GCS backend", func() {

					BeforeEach(func() {
						secret = f.SecretForGCSBackend()
						secret = f.PatchSecretForRestic(secret)
						repo = f.Repository(mongodb.ObjectMeta)
						bc = f.BackupConfiguration(mongodb.ObjectMeta, repo)

						repo.Spec.Backend = store.Backend{
							GCS: &store.GCSSpec{
								Bucket: os.Getenv("GCS_BUCKET_NAME"),
								Prefix: fmt.Sprintf("stash/%v/%v", mongodb.Namespace, mongodb.Name),
							},
							StorageSecretName: secret.Name,
						}
					})

					Context("-", func() {
						BeforeEach(func() {
							mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
							anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
						})
						It("should run successfully", shouldInitializeFromStash)
					})

					Context("Standalone with SSL", func() {

						Context("with requireSSL sslMode", func() {
							BeforeEach(func() {
								mongodb.Spec.SSLMode = api.SSLModeRequireSSL
								anotherMongoDB.Spec.SSLMode = api.SSLModeRequireSSL

								mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
								anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
							})

							It("should initialize database successfully", shouldInitializeFromStash)
						})

						Context("with allowSSL sslMode", func() {
							BeforeEach(func() {
								mongodb.Spec.SSLMode = api.SSLModeAllowSSL
								anotherMongoDB.Spec.SSLMode = api.SSLModeAllowSSL

								mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
								anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
							})

							It("should initialize database successfully", shouldInitializeFromStash)
						})

						Context("with preferSSL sslMode", func() {
							BeforeEach(func() {
								mongodb.Spec.SSLMode = api.SSLModePreferSSL
								anotherMongoDB.Spec.SSLMode = api.SSLModePreferSSL

								mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
								anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
							})

							It("should initialize database successfully", shouldInitializeFromStash)
						})
					})

					Context("With Replica Set", func() {
						BeforeEach(func() {
							mongodb = f.MongoDBRS()
							anotherMongoDB = f.MongoDBRS()
							secret = f.SecretForGCSBackend()
							secret = f.PatchSecretForRestic(secret)
							repo = f.Repository(mongodb.ObjectMeta)
							bc = f.BackupConfiguration(mongodb.ObjectMeta, repo)

							repo.Spec.Backend = store.Backend{
								GCS: &store.GCSSpec{
									Bucket: os.Getenv("GCS_BUCKET_NAME"),
									Prefix: fmt.Sprintf("stash/%v/%v", mongodb.Namespace, mongodb.Name),
								},
								StorageSecretName: secret.Name,
							}
						})

						Context("-", func() {
							BeforeEach(func() {
								mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
								anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
							})
							It("should take Snapshot successfully", shouldInitializeFromStash)
						})

						Context("with SSL", func() {

							Context("with requireSSL sslMode", func() {
								BeforeEach(func() {
									mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeX509
									mongodb.Spec.SSLMode = api.SSLModeRequireSSL

									anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeX509
									anotherMongoDB.Spec.SSLMode = api.SSLModeRequireSSL

									mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
									anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
								})

								It("should initialize database successfully", shouldInitializeFromStash)
							})

							Context("with allowSSL sslMode", func() {
								BeforeEach(func() {
									mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeKeyFile
									mongodb.Spec.SSLMode = api.SSLModeAllowSSL

									anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeKeyFile
									anotherMongoDB.Spec.SSLMode = api.SSLModeAllowSSL

									mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
									anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
								})

								It("should initialize database successfully", shouldInitializeFromStash)
							})

							Context("with preferSSL sslMode", func() {
								BeforeEach(func() {
									mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeX509
									mongodb.Spec.SSLMode = api.SSLModePreferSSL

									anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeX509
									anotherMongoDB.Spec.SSLMode = api.SSLModePreferSSL

									mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
									anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
								})

								It("should initialize database successfully", shouldInitializeFromStash)
							})
						})
					})

					Context("With Sharding", func() {
						BeforeEach(func() {
							verifySharding = true
							anotherMongoDB = f.MongoDBShard()
							mongodb = f.MongoDBShard()
							secret = f.SecretForGCSBackend()
							secret = f.PatchSecretForRestic(secret)
							repo = f.Repository(mongodb.ObjectMeta)
							bc = f.BackupConfiguration(mongodb.ObjectMeta, repo)

							repo.Spec.Backend = store.Backend{
								GCS: &store.GCSSpec{
									Bucket: os.Getenv("GCS_BUCKET_NAME"),
									Prefix: fmt.Sprintf("stash/%v/%v", mongodb.Namespace, mongodb.Name),
								},
								StorageSecretName: secret.Name,
							}
						})

						Context("With Sharding disabled database", func() {
							BeforeEach(func() {
								enableSharding = false
								mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
								anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
							})

							It("should initialize database successfully", shouldInitializeFromStash)
						})

						Context("With Sharding Enabled database", func() {
							BeforeEach(func() {
								enableSharding = true
								mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
								anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
							})

							It("should initialize database successfully", shouldInitializeFromStash)
						})

						Context("with SSL", func() {
							BeforeEach(func() {
								enableSharding = true
							})

							Context("with requireSSL sslMode", func() {
								BeforeEach(func() {
									mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeX509
									mongodb.Spec.SSLMode = api.SSLModeRequireSSL

									anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeX509
									anotherMongoDB.Spec.SSLMode = api.SSLModeRequireSSL

									mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
									anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
								})

								It("should initialize database successfully", shouldInitializeFromStash)
							})

							Context("with allowSSL sslMode", func() {
								BeforeEach(func() {
									mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeKeyFile
									mongodb.Spec.SSLMode = api.SSLModeAllowSSL

									anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeKeyFile
									anotherMongoDB.Spec.SSLMode = api.SSLModeAllowSSL

									mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
									anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
								})

								It("should initialize database successfully", shouldInitializeFromStash)
							})

							Context("with preferSSL sslMode", func() {
								BeforeEach(func() {
									mongodb.Spec.ClusterAuthMode = api.ClusterAuthModeX509
									mongodb.Spec.SSLMode = api.SSLModePreferSSL

									anotherMongoDB.Spec.ClusterAuthMode = api.ClusterAuthModeX509
									anotherMongoDB.Spec.SSLMode = api.SSLModePreferSSL

									mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
									anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
								})

								It("should initialize database successfully", shouldInitializeFromStash)
							})
						})
					})

					Context("From Sharding to standalone", func() {
						var customAppBindingName string
						BeforeEach(func() {
							enableSharding = true
							mongodb = f.MongoDBShard()
							anotherMongoDB = f.MongoDBStandalone()
							customAppBindingName = mongodb.Name + "custom"
							repo = f.Repository(mongodb.ObjectMeta)
							bc = f.BackupConfiguration(mongodb.ObjectMeta, repo)

							repo.Spec.Backend = store.Backend{
								GCS: &store.GCSSpec{
									Bucket: os.Getenv("GCS_BUCKET_NAME"),
									Prefix: fmt.Sprintf("stash/%v/%v", mongodb.Namespace, mongodb.Name),
								},
								StorageSecretName: secret.Name,
							}

							mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
							anotherMongoDB = f.MongoDBWithFlexibleProbeTimeout(anotherMongoDB)
						})

						AfterEach(func() {
							By("Deleting custom AppBindig")
							err = f.DeleteAppBinding(metav1.ObjectMeta{Name: customAppBindingName, Namespace: mongodb.Namespace})
							Expect(err).NotTo(HaveOccurred())
						})

						It("should take Snapshot successfully", func() {
							// Create and wait for running MongoDB
							createAndWaitForRunning()

							if enableSharding {
								By("Enable sharding for db:" + dbName)
								f.EventuallyEnableSharding(mongodb.ObjectMeta, dbName).Should(BeTrue())
							}
							if verifySharding {
								By("Check if db " + dbName + " is set to partitioned")
								f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
							}

							By("Insert Document Inside DB")
							f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 50).Should(BeTrue())

							By("Checking Inserted Document")
							f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 50).Should(BeTrue())

							err = f.EnsureCustomAppBinding(mongodb, customAppBindingName)
							Expect(err).NotTo(HaveOccurred())

							By("Create Secret")
							err = f.CreateSecret(secret)
							Expect(err).NotTo(HaveOccurred())

							By("Create Repositories")
							err = f.CreateRepository(repo)
							Expect(err).NotTo(HaveOccurred())

							bc.Spec.Target.Ref.Name = customAppBindingName

							By("Create BackupConfiguration")
							err = f.CreateBackupConfiguration(bc)
							Expect(err).NotTo(HaveOccurred())

							By("Check for snapshot count in stash-repository")
							f.EventuallySnapshotInRepository(repo.ObjectMeta).Should(matcher.MoreThan(2))

							By("Delete BackupConfiguration to stop backup scheduling")
							err = f.DeleteBackupConfiguration(bc.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())

							oldMongoDB, err := f.GetMongoDB(mongodb.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())

							garbageMongoDB.Items = append(garbageMongoDB.Items, *oldMongoDB)

							By("Create mongodb from stash")
							mongodb = anotherMongoDB // without value?
							rs = f.RestoreSession(mongodb.ObjectMeta, repo)
							mongodb.Spec.DatabaseSecret = oldMongoDB.Spec.DatabaseSecret
							mongodb.Spec.Init = &api.InitSpec{
								StashRestoreSession: &core.LocalObjectReference{
									Name: rs.Name,
								},
							}

							// Create and wait for running MongoDB
							createAndWaitForInitializing()

							By("Create RestoreSession")
							err = f.CreateRestoreSession(rs)
							Expect(err).NotTo(HaveOccurred())

							// eventually backupsession succeeded
							By("Check for Succeeded restoreSession")
							f.EventuallyRestoreSessionPhase(rs.ObjectMeta).Should(Equal(stashV1beta1.RestoreSessionSucceeded))

							By("Wait for Running mongodb")
							f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

							if verifySharding && mongodb.Spec.ShardTopology != nil {
								By("Check if db " + dbName + " is set to partitioned")
								f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
							}

							By("Checking previously Inserted Document")
							f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 50).Should(BeTrue())
						})
					})
				})
			})
		})

		Context("Resume", func() {
			var usedInitScript bool
			BeforeEach(func() {
				usedInitScript = false
			})

			Context("Super Fast User - Create-Delete-Create-Delete-Create ", func() {
				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be deleted")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					// Delete without caring if DB is resumed
					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for MongoDB to be deleted")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Ping mongodb database")
					f.EventuallyPingMongo(mongodb.ObjectMeta)

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					_, err = f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("Without Init", func() {

				var shouldResumeWithoutInit = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be paused")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Ping mongodb database")
					f.EventuallyPingMongo(mongodb.ObjectMeta)

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					_, err = f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				}

				It("should resume DormantDatabase successfully", shouldResumeWithoutInit)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
					})
					It("should take Snapshot successfully", shouldResumeWithoutInit)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})
					It("should take Snapshot successfully", shouldResumeWithoutInit)
				})
			})

			Context("with init Script", func() {
				var configMap *core.ConfigMap

				BeforeEach(func() {
					usedInitScript = true

					configMap = f.ConfigMapForInitialization()
					err := f.CreateConfigMap(configMap)
					Expect(err).NotTo(HaveOccurred())

					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					}
				})

				AfterEach(func() {
					err := f.DeleteConfigMap(configMap.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				var shouldResumeWithInit = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be paused")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Ping mongodb database")
					f.EventuallyPingMongo(mongodb.ObjectMeta)

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					mg, err := f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					*mongodb = *mg
					if usedInitScript {
						Expect(mongodb.Spec.Init).ShouldNot(BeNil())
						_, err := meta_util.GetString(mongodb.Annotations, api.AnnotationInitialized)
						Expect(err).To(HaveOccurred())
					}
				}

				It("should resume DormantDatabase successfully", shouldResumeWithInit)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
					})
					It("should take Snapshot successfully", shouldResumeWithInit)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
						mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})
					It("should take Snapshot successfully", shouldResumeWithInit)
				})
			})

			Context("Multiple times with init script", func() {
				var configMap *core.ConfigMap

				BeforeEach(func() {
					usedInitScript = true

					configMap = f.ConfigMapForInitialization()
					err := f.CreateConfigMap(configMap)
					Expect(err).NotTo(HaveOccurred())

					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					}
				})

				AfterEach(func() {
					err := f.DeleteConfigMap(configMap.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				var shouldResumeMultipleTimes = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					for i := 0; i < 3; i++ {
						By(fmt.Sprintf("%v-th", i+1) + " time running.")
						By("Delete mongodb")
						err = f.DeleteMongoDB(mongodb.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for mongodb to be paused")
						f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

						// Create MongoDB object again to resume it
						By("Create MongoDB: " + mongodb.Name)
						err = f.CreateMongoDB(mongodb)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for Running mongodb")
						f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

						_, err := f.GetMongoDB(mongodb.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Ping mongodb database")
						f.EventuallyPingMongo(mongodb.ObjectMeta)

						By("Checking Inserted Document")
						f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

						if usedInitScript {
							Expect(mongodb.Spec.Init).ShouldNot(BeNil())
							_, err := meta_util.GetString(mongodb.Annotations, api.AnnotationInitialized)
							Expect(err).To(HaveOccurred())
						}
					}
				}

				It("should resume DormantDatabase successfully", shouldResumeMultipleTimes)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
					})
					It("should take Snapshot successfully", shouldResumeMultipleTimes)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
						mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})
					It("should take Snapshot successfully", shouldResumeMultipleTimes)
				})
			})

		})

		Context("Termination Policy", func() {
			BeforeEach(func() {
				secret = f.SecretForS3Backend()
			})

			var shouldRunWithSnapshot = func() {
				// Create and wait for running MongoDB
				createAndWaitForRunning()

				By("Insert Document Inside DB")
				f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

				By("Checking Inserted Document")
				f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

				By("Create Secret")
				err := f.CreateSecret(secret)
				Expect(err).NotTo(HaveOccurred())
			}

			Context("with TerminationDoNotTerminate", func() {
				BeforeEach(func() {
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
				})

				var shouldWorkDoNotTerminate = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).Should(HaveOccurred())

					By("MongoDB is not paused. Check for mongodb")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeTrue())

					By("Check for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Update mongodb to set spec.terminationPolicy = Pause")
					_, err := f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.TerminationPolicy = api.TerminationPolicyHalt
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				}

				It("should work successfully", shouldWorkDoNotTerminate)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
					})
					It("should run successfully", shouldWorkDoNotTerminate)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyDoNotTerminate
						//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})
					It("should run successfully", shouldWorkDoNotTerminate)
				})

			})

			Context("with TerminationPolicyHalt", func() {
				var shouldRunWithTerminationPause = func() {
					shouldRunWithSnapshot()
					By("Halt MongoDB: Update mongodb to set spec.halted = true")
					_, err := f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.Halted = true
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for halted/paused mongodb")
					f.EventuallyMongoDBPhase(mongodb.ObjectMeta).Should(Equal(api.DatabasePhasePaused))

					By("Resume MongoDB: Update mongodb to set spec.halted = false")
					_, err = f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.Halted = false
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					mongodb, err = f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Ping mongodb database")
					f.EventuallyPingMongo(mongodb.ObjectMeta)

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

				}

				It("should create dormantdatabase successfully", shouldRunWithTerminationPause)

				Context("with Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
					})

					It("should create dormantdatabase successfully", shouldRunWithTerminationPause)
				})

				Context("with Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})

					It("should create dormantdatabase successfully", shouldRunWithTerminationPause)
				})
			})

			Context("with TerminationPolicyDelete", func() {
				BeforeEach(func() {
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyDelete
				})

				var shouldRunWithTerminationDelete = func() {
					shouldRunWithSnapshot()

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("wait until mongodb is deleted")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

					By("Check for deleted PVCs")
					f.EventuallyPVCCount(mongodb.ObjectMeta).Should(Equal(0))

					By("Check for intact Secrets")
					f.EventuallyDBSecretCount(mongodb.ObjectMeta).ShouldNot(Equal(0))
				}

				It("should run with TerminationPolicyDelete", shouldRunWithTerminationDelete)

				Context("with Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyDelete
					})
					It("should initialize database successfully", shouldRunWithTerminationDelete)
				})

				Context("with Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyDelete
					})
					It("should initialize database successfully", shouldRunWithTerminationDelete)
				})
			})

			Context("with TerminationPolicyWipeOut", func() {
				BeforeEach(func() {
					mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				})

				var shouldRunWithTerminationWipeOut = func() {
					shouldRunWithSnapshot()

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("wait until mongodb is deleted")
					f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

					By("Check for deleted PVCs")
					f.EventuallyPVCCount(mongodb.ObjectMeta).Should(Equal(0))

					By("Check for deleted Secrets")
					f.EventuallyDBSecretCount(mongodb.ObjectMeta).Should(Equal(0))
				}

				It("should run with TerminationPolicyWipeOut", shouldRunWithTerminationWipeOut)

				Context("with Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					It("should initialize database successfully", shouldRunWithTerminationWipeOut)
				})

				Context("with Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

						//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})
					It("should initialize database successfully", shouldRunWithTerminationWipeOut)
				})
			})
		})

		Context("Environment Variables", func() {

			Context("With allowed Envs", func() {
				var configMap *core.ConfigMap

				BeforeEach(func() {
					configMap = f.ConfigMapForInitialization()
					err := f.CreateConfigMap(configMap)
					Expect(err).NotTo(HaveOccurred())

					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					}
				})

				AfterEach(func() {
					err := f.DeleteConfigMap(configMap.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				var withAllowedEnvs = func() {
					dbName = "envDB"
					envs := []core.EnvVar{
						{
							Name:  MONGO_INITDB_DATABASE,
							Value: dbName,
						},
					}
					if mongodb.Spec.ShardTopology != nil {
						mongodb.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
						mongodb.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
						mongodb.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

					} else {
						if mongodb.Spec.PodTemplate == nil {
							mongodb.Spec.PodTemplate = new(ofst.PodTemplateSpec)
						}
						mongodb.Spec.PodTemplate.Spec.Env = envs
					}

					// Create MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
				}

				It("should initialize database specified by env", withAllowedEnvs)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
					})
					It("should initialize database specified by env", withAllowedEnvs)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
						//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})
					It("should initialize database specified by env", withAllowedEnvs)
				})

			})

			Context("With forbidden Envs", func() {

				var withForbiddenEnvs = func() {

					By("Create MongoDB with " + MONGO_INITDB_ROOT_USERNAME + " env var")
					envs := []core.EnvVar{
						{
							Name:  MONGO_INITDB_ROOT_USERNAME,
							Value: "mg-user",
						},
					}
					if mongodb.Spec.ShardTopology != nil {
						mongodb.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
						mongodb.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
						mongodb.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

					} else {
						if mongodb.Spec.PodTemplate == nil {
							mongodb.Spec.PodTemplate = new(ofst.PodTemplateSpec)
						}
						mongodb.Spec.PodTemplate.Spec.Env = envs
					}
					err = f.CreateMongoDB(mongodb)
					Expect(err).To(HaveOccurred())

					By("Create MongoDB with " + MONGO_INITDB_ROOT_PASSWORD + " env var")
					envs = []core.EnvVar{
						{
							Name:  MONGO_INITDB_ROOT_PASSWORD,
							Value: "not@secret",
						},
					}
					if mongodb.Spec.ShardTopology != nil {
						mongodb.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
						mongodb.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
						mongodb.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

					} else {
						if mongodb.Spec.PodTemplate == nil {
							mongodb.Spec.PodTemplate = new(ofst.PodTemplateSpec)
						}
						mongodb.Spec.PodTemplate.Spec.Env = envs
					}
					err = f.CreateMongoDB(mongodb)
					Expect(err).To(HaveOccurred())
				}

				It("should reject to create MongoDB crd", withForbiddenEnvs)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
					})
					It("should take Snapshot successfully", withForbiddenEnvs)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})
					It("should take Snapshot successfully", withForbiddenEnvs)
				})
			})

			Context("Update Envs", func() {
				var configMap *core.ConfigMap

				BeforeEach(func() {
					configMap = f.ConfigMapForInitialization()
					err := f.CreateConfigMap(configMap)
					Expect(err).NotTo(HaveOccurred())

					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								ConfigMap: &core.ConfigMapVolumeSource{
									LocalObjectReference: core.LocalObjectReference{
										Name: configMap.Name,
									},
								},
							},
						},
					}
				})

				AfterEach(func() {
					err := f.DeleteConfigMap(configMap.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})

				var withUpdateEnvs = func() {

					dbName = "envDB"
					envs := []core.EnvVar{
						{
							Name:  MONGO_INITDB_DATABASE,
							Value: dbName,
						},
					}
					if mongodb.Spec.ShardTopology != nil {
						mongodb.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
						mongodb.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
						mongodb.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

					} else {
						if mongodb.Spec.PodTemplate == nil {
							mongodb.Spec.PodTemplate = new(ofst.PodTemplateSpec)
						}
						mongodb.Spec.PodTemplate.Spec.Env = envs
					}

					// Create MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

					_, _, err = util.PatchMongoDB(f.DBClient().KubedbV1alpha1(), mongodb, func(in *api.MongoDB) *api.MongoDB {
						envs = []core.EnvVar{
							{
								Name:  MONGO_INITDB_DATABASE,
								Value: "patched-db",
							},
						}
						if in.Spec.ShardTopology != nil {
							in.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
							in.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
							in.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

						} else {
							if in.Spec.PodTemplate == nil {
								in.Spec.PodTemplate = new(ofst.PodTemplateSpec)
							}
							in.Spec.PodTemplate.Spec.Env = envs
						}
						return in
					})
					Expect(err).NotTo(HaveOccurred())
				}

				It("should not reject to update EnvVar", withUpdateEnvs)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
					})

					It("should not reject to update EnvVar", withUpdateEnvs)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									ConfigMap: &core.ConfigMapVolumeSource{
										LocalObjectReference: core.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						}
						//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})

					It("should not reject to update EnvVar", withUpdateEnvs)
				})
			})
		})

		Context("Custom config", func() {

			var maxIncomingConnections = int32(10000)
			customConfigs := []string{
				fmt.Sprintf(`   maxIncomingConnections: %v`, maxIncomingConnections),
			}

			Context("from configMap", func() {
				var userConfig *core.ConfigMap

				BeforeEach(func() {
					userConfig = f.GetCustomConfig(customConfigs)
				})

				AfterEach(func() {
					By("Deleting configMap: " + userConfig.Name)
					err := f.DeleteConfigMap(userConfig.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

				})

				runWithUserProvidedConfig := func() {
					if skipMessage != "" {
						Skip(skipMessage)
					}

					By("Creating configMap: " + userConfig.Name)
					err := f.CreateConfigMap(userConfig)
					Expect(err).NotTo(HaveOccurred())

					// Create MySQL
					createAndWaitForRunning()

					By("Checking maxIncomingConnections from mongodb config")
					f.EventuallyMaxIncomingConnections(mongodb.ObjectMeta).Should(Equal(maxIncomingConnections))
				}

				Context("Standalone MongoDB", func() {

					BeforeEach(func() {
						mongodb = f.MongoDBStandalone()
						mongodb.Spec.ConfigSource = &core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: userConfig.Name,
								},
							},
						}
					})

					It("should run successfully", runWithUserProvidedConfig)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.ConfigSource = &core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: userConfig.Name,
								},
							},
						}
					})

					It("should run successfully", runWithUserProvidedConfig)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.ShardTopology.Shard.ConfigSource = &core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: userConfig.Name,
								},
							},
						}
						mongodb.Spec.ShardTopology.ConfigServer.ConfigSource = &core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: userConfig.Name,
								},
							},
						}
						mongodb.Spec.ShardTopology.Mongos.ConfigSource = &core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: userConfig.Name,
								},
							},
						}

						//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)

					})

					It("should run successfully", runWithUserProvidedConfig)
				})

			})
		})

		Context("StorageType ", func() {

			var shouldRunSuccessfully = func() {

				if skipMessage != "" {
					Skip(skipMessage)
				}
				// Create MongoDB
				createAndWaitForRunning()

				By("Insert Document Inside DB")
				f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

				By("Checking Inserted Document")
				f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
			}

			Context("Ephemeral", func() {

				Context("Standalone MongoDB", func() {

					BeforeEach(func() {
						mongodb.Spec.StorageType = api.StorageTypeEphemeral
						mongodb.Spec.Storage = nil
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})

					It("should run successfully", shouldRunSuccessfully)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Replicas = types.Int32P(3)
						mongodb.Spec.StorageType = api.StorageTypeEphemeral
						mongodb.Spec.Storage = nil
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})

					It("should run successfully", shouldRunSuccessfully)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.StorageType = api.StorageTypeEphemeral
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

						//mongodb = f.MongoDBWithFlexibleProbeTimeout(mongodb)
					})

					It("should run successfully", shouldRunSuccessfully)
				})

				Context("With TerminationPolicyHalt", func() {

					BeforeEach(func() {
						mongodb.Spec.StorageType = api.StorageTypeEphemeral
						mongodb.Spec.Storage = nil
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyHalt
					})

					It("should reject to create MongoDB object", func() {

						By("Creating MongoDB: " + mongodb.Name)
						err := f.CreateMongoDB(mongodb)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
	})
})
