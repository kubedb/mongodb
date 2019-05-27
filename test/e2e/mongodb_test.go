package e2e_test

import (
	"fmt"
	"os"

	"github.com/appscode/go/types"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/mongodb/test/e2e/framework"
	"github.com/kubedb/mongodb/test/e2e/matcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_util "kmodules.xyz/client-go/meta"
	store "kmodules.xyz/objectstore-api/api/v1"
	ofst "kmodules.xyz/offshoot-api/api/v1"
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
		err                      error
		f                        *framework.Invocation
		mongodb                  *api.MongoDB
		garbageMongoDB           *api.MongoDBList
		snapshot                 *api.Snapshot
		snapshotPVC              *core.PersistentVolumeClaim
		secret                   *core.Secret
		skipMessage              string
		skipSnapshotDataChecking bool
		verifySharding           bool
		enableSharding           bool
		dbName                   string
	)

	BeforeEach(func() {
		f = root.Invoke()
		mongodb = f.MongoDBStandalone()
		garbageMongoDB = new(api.MongoDBList)
		snapshot = f.Snapshot()
		secret = nil
		skipMessage = ""
		skipSnapshotDataChecking = true
		verifySharding = false
		enableSharding = false
		dbName = "kubedb"
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over MongoDB objects")
		root.CleanMongoDB()
		By("Delete left over Dormant Database objects")
		root.CleanDormantDatabase()
		By("Delete left over Snapshot objects")
		root.CleanSnapshot()
		By("Delete left over workloads if exists any")
		root.CleanWorkloadLeftOvers()

		if snapshotPVC != nil {
			err := f.DeletePersistentVolumeClaim(snapshotPVC.ObjectMeta)
			if err != nil && !kerr.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
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
	}

	var deleteTestResource = func() {
		if mongodb == nil {
			Skip("Skipping")
		}

		By("Check if mongodb " + mongodb.Name + " exists.")
		mg, err := f.GetMongoDB(mongodb.ObjectMeta)
		if err != nil {
			if kerr.IsNotFound(err) {
				// MongoDB was not created. Hence, rest of cleanup is not necessary.
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

		By("Delete mongodb")
		err = f.DeleteMongoDB(mongodb.ObjectMeta)
		if err != nil {
			if kerr.IsNotFound(err) {
				// MongoDB was not created. Hence, rest of cleanup is not necessary.
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

		if mg.Spec.TerminationPolicy == api.TerminationPolicyPause {

			By("Wait for mongodb to be paused")
			f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

			By("Set DormantDatabase Spec.WipeOut to true")
			_, err = f.PatchDormantDatabase(mongodb.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
				in.Spec.WipeOut = true
				return in
			})
			Expect(err).NotTo(HaveOccurred())

			By("Delete Dormant Database")
			err = f.DeleteDormantDatabase(mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		}

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

			if !skipSnapshotDataChecking {
				By("Check for snapshot data")
				f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
			}

			if secret != nil {
				err := f.DeleteSecret(secret.ObjectMeta)
				if !kerr.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		Context("General", func() {

			Context("With PVC", func() {

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
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 3).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 3).Should(BeTrue())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be paused")
					f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					if verifySharding {
						By("Check if db " + dbName + " is set to partitioned")
						f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
					}

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 3).Should(BeTrue())
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
		})

		Context("Snapshot", func() {
			BeforeEach(func() {
				skipSnapshotDataChecking = false
				snapshot.Spec.DatabaseName = mongodb.Name
			})

			var shouldTakeSnapshot = func() {
				// Create and wait for running MongoDB
				createAndWaitForRunning()

				By("Create Secret")
				err := f.CreateSecret(secret)
				Expect(err).NotTo(HaveOccurred())

				By("Create Snapshot")
				err = f.CreateSnapshot(snapshot)
				Expect(err).NotTo(HaveOccurred())

				By("Check for Succeeded snapshot")
				f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

				if !skipSnapshotDataChecking {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
				}
			}

			Context("In Local", func() {
				BeforeEach(func() {
					skipSnapshotDataChecking = true
					secret = f.SecretForLocalBackend()
					snapshot.Spec.StorageSecretName = secret.Name
				})

				Context("With EmptyDir as Snapshot's backend", func() {
					BeforeEach(func() {
						snapshot.Spec.Local = &store.LocalSpec{
							MountPath: "/repo",
							VolumeSource: core.VolumeSource{
								EmptyDir: &core.EmptyDirVolumeSource{},
							},
						}
					})

					It("should take Snapshot successfully", shouldTakeSnapshot)
				})

				Context("With PVC as Snapshot's backend", func() {

					BeforeEach(func() {
						snapshotPVC = f.GetPersistentVolumeClaim()
						By("Creating PVC for local backend snapshot")
						err := f.CreatePersistentVolumeClaim(snapshotPVC)
						Expect(err).NotTo(HaveOccurred())

						snapshot.Spec.Local = &store.LocalSpec{
							MountPath: "/repo",
							VolumeSource: core.VolumeSource{
								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
									ClaimName: snapshotPVC.Name,
								},
							},
						}
					})

					It("should delete Snapshot successfully", func() {
						shouldTakeSnapshot()

						By("Deleting Snapshot")
						err := f.DeleteSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting Snapshot to be deleted")
						f.EventuallySnapshot(snapshot.ObjectMeta).Should(BeFalse())
					})
				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						snapshot.Spec.DatabaseName = mongodb.Name
						snapshot.Spec.Local = &store.LocalSpec{
							MountPath: "/repo",
							VolumeSource: core.VolumeSource{
								EmptyDir: &core.EmptyDirVolumeSource{},
							},
						}
					})
					It("should take Snapshot successfully", shouldTakeSnapshot)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						snapshot.Spec.DatabaseName = mongodb.Name
						snapshot.Spec.Local = &store.LocalSpec{
							MountPath: "/repo",
							VolumeSource: core.VolumeSource{
								EmptyDir: &core.EmptyDirVolumeSource{},
							},
						}
					})
					It("should take Snapshot successfully", shouldTakeSnapshot)
				})
			})

			Context("In S3", func() {
				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &store.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				Context("Faulty Snapshot", func() {
					BeforeEach(func() {
						skipSnapshotDataChecking = true
						snapshot.Spec.S3 = &store.S3Spec{
							Bucket: "nonexisting",
						}
					})
					It("snapshot should fail", func() {
						// Create and wait for running MongoDB
						createAndWaitForRunning()

						By("Create Secret")
						err := f.CreateSecret(secret)
						Expect(err).NotTo(HaveOccurred())

						By("Create Snapshot")
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Failed snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseFailed))
					})
				})

				Context("Delete One Snapshot keeping others", func() {

					BeforeEach(func() {
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})

					It("Delete One Snapshot keeping others", func() {
						// Create and wait for running MongoDB
						shouldTakeSnapshot()

						oldSnapshot := snapshot.DeepCopy()

						// New snapshot that has old snapshot's name in prefix
						snapshot.Name += "-2"

						By(fmt.Sprintf("Create Snapshot %v", snapshot.Name))
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Succeeded snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

						if !skipSnapshotDataChecking {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						// delete old snapshot
						By(fmt.Sprintf("Delete old Snapshot %v", oldSnapshot.Name))
						err = f.DeleteSnapshot(oldSnapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for old Snapshot to be deleted")
						f.EventuallySnapshot(oldSnapshot.ObjectMeta).Should(BeFalse())
						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for old snapshot %v", oldSnapshot.Name))
							f.EventuallySnapshotDataFound(oldSnapshot).Should(BeFalse())
						}

						// check remaining snapshot
						By(fmt.Sprintf("Checking another Snapshot %v still exists", snapshot.Name))
						_, err = f.GetSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for remaining snapshot %v", snapshot.Name))
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}
					})
				})
			})

			Context("In GCS", func() {
				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &store.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Replicas = types.Int32P(3)
						snapshot.Spec.DatabaseName = mongodb.Name
					})
					It("should take Snapshot successfully", shouldTakeSnapshot)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						snapshot.Spec.DatabaseName = mongodb.Name
					})
					It("should take Snapshot successfully", shouldTakeSnapshot)
				})

				Context("Delete One Snapshot keeping others", func() {

					BeforeEach(func() {
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})

					It("Delete One Snapshot keeping others", func() {
						// Create and wait for running MongoDB
						shouldTakeSnapshot()

						oldSnapshot := snapshot.DeepCopy()

						// New snapshot that has old snapshot's name in prefix
						snapshot.Name += "-2"

						By(fmt.Sprintf("Create Snapshot %v", snapshot.Name))
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Succeeded snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

						if !skipSnapshotDataChecking {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						// delete old snapshot
						By(fmt.Sprintf("Delete old Snapshot %v", oldSnapshot.Name))
						err = f.DeleteSnapshot(oldSnapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for old Snapshot to be deleted")
						f.EventuallySnapshot(oldSnapshot.ObjectMeta).Should(BeFalse())
						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for old snapshot %v", oldSnapshot.Name))
							f.EventuallySnapshotDataFound(oldSnapshot).Should(BeFalse())
						}

						// check remaining snapshot
						By(fmt.Sprintf("Checking another Snapshot %v still exists", snapshot.Name))
						_, err = f.GetSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for remaining snapshot %v", snapshot.Name))
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}
					})
				})
			})

			Context("In Azure", func() {
				BeforeEach(func() {
					secret = f.SecretForAzureBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Azure = &store.AzureSpec{
						Container: os.Getenv(AZURE_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				Context("Delete One Snapshot keeping others", func() {

					BeforeEach(func() {
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})

					It("Delete One Snapshot keeping others", func() {
						// Create and wait for running MongoDB
						shouldTakeSnapshot()

						oldSnapshot := snapshot.DeepCopy()

						// New snapshot that has old snapshot's name in prefix
						snapshot.Name += "-2"

						By(fmt.Sprintf("Create Snapshot %v", snapshot.Name))
						err = f.CreateSnapshot(snapshot)
						Expect(err).NotTo(HaveOccurred())

						By("Check for Succeeded snapshot")
						f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

						if !skipSnapshotDataChecking {
							By("Check for snapshot data")
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}

						// delete old snapshot
						By(fmt.Sprintf("Delete old Snapshot %v", oldSnapshot.Name))
						err = f.DeleteSnapshot(oldSnapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Waiting for old Snapshot to be deleted")
						f.EventuallySnapshot(oldSnapshot.ObjectMeta).Should(BeFalse())
						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for old snapshot %v", oldSnapshot.Name))
							f.EventuallySnapshotDataFound(oldSnapshot).Should(BeFalse())
						}

						// check remaining snapshot
						By(fmt.Sprintf("Checking another Snapshot %v still exists", snapshot.Name))
						_, err = f.GetSnapshot(snapshot.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						if !skipSnapshotDataChecking {
							By(fmt.Sprintf("Check data for remaining snapshot %v", snapshot.Name))
							f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
						}
					})
				})
			})

			Context("In Swift", func() {
				BeforeEach(func() {
					secret = f.SecretForSwiftBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Swift = &store.SwiftSpec{
						Container: os.Getenv(SWIFT_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("Snapshot PodVolume Template - In S3", func() {

				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &store.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
				})

				var shouldHandleJobVolumeSuccessfully = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Get MongoDB")
					es, err := f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
					mongodb.Spec = es.Spec

					By("Create Secret")
					err = f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					// determine pvcSpec and storageType for job
					// start
					pvcSpec := snapshot.Spec.PodVolumeClaimSpec
					if pvcSpec == nil {
						pvcSpec = mongodb.Spec.Storage
					}
					st := snapshot.Spec.StorageType
					if st == nil {
						st = &mongodb.Spec.StorageType
					}
					Expect(st).NotTo(BeNil())
					// end

					By("Create Snapshot")
					err = f.CreateSnapshot(snapshot)
					if *st == api.StorageTypeDurable && pvcSpec == nil {
						By("Create Snapshot should have failed")
						Expect(err).Should(HaveOccurred())
						return
					} else {
						Expect(err).NotTo(HaveOccurred())
					}

					By("Get Snapshot")
					snap, err := f.GetSnapshot(snapshot.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
					snapshot.Spec = snap.Spec

					if *st == api.StorageTypeEphemeral {
						storageSize := "0"
						if pvcSpec != nil {
							if sz, found := pvcSpec.Resources.Requests[core.ResourceStorage]; found {
								storageSize = sz.String()
							}
						}
						By(fmt.Sprintf("Check for Job Empty volume size: %v", storageSize))
						f.EventuallyJobVolumeEmptyDirSize(snapshot.ObjectMeta).Should(Equal(storageSize))
					} else if *st == api.StorageTypeDurable {
						sz, found := pvcSpec.Resources.Requests[core.ResourceStorage]
						Expect(found).NotTo(BeFalse())

						By("Check for Job PVC Volume size from snapshot")
						f.EventuallyJobPVCSize(snapshot.ObjectMeta).Should(Equal(sz.String()))
					}

					By("Check for succeeded snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

					if !skipSnapshotDataChecking {
						By("Check for snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
					}
				}

				// db StorageType Scenarios
				// ==============> Start
				var dbStorageTypeScenarios = func() {
					Context("DBStorageType - Durable", func() {
						BeforeEach(func() {
							mongodb.Spec.StorageType = api.StorageTypeDurable
							mongodb.Spec.Storage = &core.PersistentVolumeClaimSpec{
								Resources: core.ResourceRequirements{
									Requests: core.ResourceList{
										core.ResourceStorage: resource.MustParse(framework.DBPvcStorageSize),
									},
								},
								StorageClassName: types.StringP(root.StorageClass),
							}

						})

						It("should Handle Job Volume Successfully", shouldHandleJobVolumeSuccessfully)
					})

					Context("DBStorageType - Ephemeral", func() {
						BeforeEach(func() {
							mongodb.Spec.StorageType = api.StorageTypeEphemeral
							mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
						})

						Context("DBPvcSpec is nil", func() {
							BeforeEach(func() {
								mongodb.Spec.Storage = nil
							})

							It("should Handle Job Volume Successfully", shouldHandleJobVolumeSuccessfully)
						})

						Context("DBPvcSpec is given [not nil]", func() {
							BeforeEach(func() {
								mongodb.Spec.Storage = &core.PersistentVolumeClaimSpec{
									Resources: core.ResourceRequirements{
										Requests: core.ResourceList{
											core.ResourceStorage: resource.MustParse(framework.DBPvcStorageSize),
										},
									},
									StorageClassName: types.StringP(root.StorageClass),
								}
							})

							It("should Handle Job Volume Successfully", shouldHandleJobVolumeSuccessfully)
						})
					})
				}
				// End <==============

				// Snapshot PVC Scenarios
				// ==============> Start
				var snapshotPvcScenarios = func() {
					Context("Snapshot PVC is given [not nil]", func() {
						BeforeEach(func() {
							snapshot.Spec.PodVolumeClaimSpec = &core.PersistentVolumeClaimSpec{
								Resources: core.ResourceRequirements{
									Requests: core.ResourceList{
										core.ResourceStorage: resource.MustParse(framework.JobPvcStorageSize),
									},
								},
								StorageClassName: types.StringP(root.StorageClass),
							}
						})

						dbStorageTypeScenarios()
					})

					Context("Snapshot PVC is nil", func() {
						BeforeEach(func() {
							snapshot.Spec.PodVolumeClaimSpec = nil
						})

						dbStorageTypeScenarios()
					})
				}
				// End <==============

				Context("Snapshot StorageType is nil", func() {
					BeforeEach(func() {
						snapshot.Spec.StorageType = nil
					})

					snapshotPvcScenarios()
				})

				Context("Snapshot StorageType is Ephemeral", func() {
					BeforeEach(func() {
						ephemeral := api.StorageTypeEphemeral
						snapshot.Spec.StorageType = &ephemeral
					})

					snapshotPvcScenarios()
				})

				Context("Snapshot StorageType is Durable", func() {
					BeforeEach(func() {
						durable := api.StorageTypeDurable
						snapshot.Spec.StorageType = &durable
					})

					snapshotPvcScenarios()
				})
			})
		})

		Context("Initialize", func() {

			Context("With Script", func() {
				BeforeEach(func() {
					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
				})

				It("should run successfully", func() {
					// Create MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())
				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Replicas = types.Int32P(3)
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})
					It("should Initialize successfully", func() {
						// Create MongoDB
						createAndWaitForRunning()

						By("Checking Inserted Document")
						f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())
					})
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})
					It("should Initialize successfully", func() {
						// Create MongoDB
						createAndWaitForRunning()

						By("Checking Inserted Document")
						f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())
					})
				})

			})

			Context("With Snapshot", func() {

				var anotherMongoDB *api.MongoDB
				var skipConfig bool

				BeforeEach(func() {
					skipConfig = true
					anotherMongoDB = f.MongoDBStandalone()
					anotherMongoDB.Spec.Init = &api.InitSpec{
						SnapshotSource: &api.SnapshotSourceSpec{
							Namespace: snapshot.Namespace,
							Name:      snapshot.Name,
						},
					}
					skipSnapshotDataChecking = false
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &store.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = mongodb.Name
				})

				var shouldInitializeSnapshot = func() {
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
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 50).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 50).Should(BeTrue())

					By("Create Secret")
					err := f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					By("Create Snapshot")
					err = f.CreateSnapshot(snapshot)
					Expect(err).NotTo(HaveOccurred())

					By("Check for Succeeded snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

					if !skipSnapshotDataChecking {
						By("Check for snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
					}

					oldMongoDB, err := f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					garbageMongoDB.Items = append(garbageMongoDB.Items, *oldMongoDB)

					By("Create mongodb from snapshot")
					mongodb = anotherMongoDB
					mongodb.Spec.DatabaseSecret = oldMongoDB.Spec.DatabaseSecret

					// Create and wait for running MongoDB
					createAndWaitForRunning()

					if verifySharding {
						By("Check if db " + dbName + " is set to partitioned")
						f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(!skipConfig))
					}

					By("Checking previously Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 50).Should(BeTrue())
				}

				It("should run successfully", shouldInitializeSnapshot)

				Context("with local volume", func() {

					BeforeEach(func() {
						snapshotPVC = f.GetPersistentVolumeClaim()
						By("Creating PVC for local backend snapshot")
						err := f.CreatePersistentVolumeClaim(snapshotPVC)
						Expect(err).NotTo(HaveOccurred())

						skipSnapshotDataChecking = true
						secret = f.SecretForLocalBackend()
						snapshot.Spec.StorageSecretName = secret.Name
						snapshot.Spec.Backend = store.Backend{
							Local: &store.LocalSpec{
								MountPath: "/repo",
								VolumeSource: core.VolumeSource{
									PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
										ClaimName: snapshotPVC.Name,
									},
								},
							},
						}
					})

					It("should initialize database successfully", shouldInitializeSnapshot)

				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						snapshot.Spec.DatabaseName = mongodb.Name
						anotherMongoDB = f.MongoDBRS()
						anotherMongoDB.Spec.Init = &api.InitSpec{
							SnapshotSource: &api.SnapshotSourceSpec{
								Namespace: snapshot.Namespace,
								Name:      snapshot.Name,
							},
						}
					})
					It("should initialize database successfully", shouldInitializeSnapshot)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						verifySharding = true
						mongodb = f.MongoDBShard()
						snapshot.Spec.DatabaseName = mongodb.Name
						anotherMongoDB = f.MongoDBShard()
						anotherMongoDB.Spec.Init = &api.InitSpec{
							SnapshotSource: &api.SnapshotSourceSpec{
								Namespace: snapshot.Namespace,
								Name:      snapshot.Name,
							},
						}
					})
					Context("-", func() {
						BeforeEach(func() {
							enableSharding = false
							skipConfig = true
							anotherMongoDB.Spec.Init = &api.InitSpec{
								SnapshotSource: &api.SnapshotSourceSpec{
									Namespace: snapshot.Namespace,
									Name:      snapshot.Name,
									Args:      []string{fmt.Sprintf("--skip-config=%v", skipConfig)},
								},
							}
						})
						It("should initialize database successfully", shouldInitializeSnapshot)
					})

					Context("With Sharding Enabled database", func() {
						BeforeEach(func() {
							enableSharding = true
							skipConfig = true
							anotherMongoDB.Spec.Init = &api.InitSpec{
								SnapshotSource: &api.SnapshotSourceSpec{
									Namespace: snapshot.Namespace,
									Name:      snapshot.Name,
									Args:      []string{fmt.Sprintf("--skip-config=%v", skipConfig)},
								},
							}
						})
						It("should initialize database successfully", shouldInitializeSnapshot)
					})

					Context("With ShardingEnabled database - skipConfig is set false", func() {
						BeforeEach(func() {
							enableSharding = true
							skipConfig = false
							anotherMongoDB.Spec.Init = &api.InitSpec{
								SnapshotSource: &api.SnapshotSourceSpec{
									Namespace: snapshot.Namespace,
									Name:      snapshot.Name,
									Args:      []string{fmt.Sprintf("--skip-config=%v", skipConfig)},
								},
							}
						})
						It("should initialize database successfully", shouldInitializeSnapshot)
					})

				})

				Context("From Sharding to standalone", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						snapshot.Spec.DatabaseName = mongodb.Name
						anotherMongoDB = f.MongoDBStandalone()
						anotherMongoDB.Spec.Init = &api.InitSpec{
							SnapshotSource: &api.SnapshotSourceSpec{
								Namespace: snapshot.Namespace,
								Name:      snapshot.Name,
								Args:      []string{"--skip-config=true"},
							},
						}
					})
					It("should take Snapshot successfully", shouldInitializeSnapshot)
				})
			})
		})

		Context("Resume", func() {
			var usedInitScript bool
			var usedInitSnapshot bool
			BeforeEach(func() {
				usedInitScript = false
				usedInitSnapshot = false
			})

			Context("Super Fast User - Create-Delete-Create-Delete-Create ", func() {
				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be paused")
					f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

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

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					_, err = f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("Without Init", func() {

				var shouldResumeWithoutInit = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be paused")
					f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

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
					})
					It("should take Snapshot successfully", shouldResumeWithoutInit)
				})
			})

			Context("with init Script", func() {
				BeforeEach(func() {
					usedInitScript = true
					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
				})

				var shouldResumeWithInit = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be paused")
					f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

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
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
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
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})
					It("should take Snapshot successfully", shouldResumeWithInit)
				})
			})

			Context("With Snapshot Init", func() {

				var anotherMongoDB *api.MongoDB

				BeforeEach(func() {
					anotherMongoDB = f.MongoDBStandalone()
					usedInitSnapshot = true
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &store.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = mongodb.Name
				})
				var shouldResumeWithSnapshot = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Create Secret")
					err := f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					By("Create Snapshot")
					err = f.CreateSnapshot(snapshot)
					Expect(err).NotTo(HaveOccurred())

					By("Check for Succeeded snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())

					oldMongoDB, err := f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					garbageMongoDB.Items = append(garbageMongoDB.Items, *oldMongoDB)

					By("Create mongodb from snapshot")
					mongodb = anotherMongoDB
					mongodb.Spec.Init = &api.InitSpec{
						SnapshotSource: &api.SnapshotSourceSpec{
							Namespace: snapshot.Namespace,
							Name:      snapshot.Name,
						},
					}

					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be paused")
					f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					mongodb, err = f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					if usedInitSnapshot {
						_, err = meta_util.GetString(mongodb.Annotations, api.AnnotationInitialized)
						Expect(err).NotTo(HaveOccurred())
					}
				}

				It("should resume successfully", shouldResumeWithSnapshot)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						snapshot.Spec.DatabaseName = mongodb.Name
						anotherMongoDB = f.MongoDBRS()
					})
					It("should take Snapshot successfully", shouldResumeWithSnapshot)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						snapshot.Spec.DatabaseName = mongodb.Name
						anotherMongoDB = f.MongoDBShard()
					})
					It("should take Snapshot successfully", shouldResumeWithSnapshot)
				})
			})

			Context("Multiple times with init script", func() {
				BeforeEach(func() {
					usedInitScript = true
					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
				})

				var shouldResumeMultipleTimes = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					for i := 0; i < 3; i++ {
						By(fmt.Sprintf("%v-th", i+1) + " time running.")
						By("Delete mongodb")
						err = f.DeleteMongoDB(mongodb.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for mongodb to be paused")
						f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

						// Create MongoDB object again to resume it
						By("Create MongoDB: " + mongodb.Name)
						err = f.CreateMongoDB(mongodb)
						Expect(err).NotTo(HaveOccurred())

						By("Wait for DormantDatabase to be deleted")
						f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

						By("Wait for Running mongodb")
						f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

						_, err := f.GetMongoDB(mongodb.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())

						By("Checking Inserted Document")
						f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

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
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
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
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})
					It("should take Snapshot successfully", shouldResumeMultipleTimes)
				})
			})

		})

		Context("SnapshotScheduler", func() {

			Context("With Startup", func() {

				var shouldStartupSchedular = func() {
					By("Create Secret")
					err := f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Count multiple Snapshot Object")
					f.EventuallySnapshotCount(mongodb.ObjectMeta).Should(matcher.MoreThan(3))

					By("Remove Backup Scheduler from MongoDB")
					_, err = f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.BackupSchedule = nil
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Verify multiple Succeeded Snapshot")
					f.EventuallyMultipleSnapshotFinishedProcessing(mongodb.ObjectMeta).Should(Succeed())
				}

				Context("with local", func() {
					BeforeEach(func() {
						secret = f.SecretForLocalBackend()
						mongodb.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 20s",
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Local: &store.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										EmptyDir: &core.EmptyDirVolumeSource{},
									},
								},
							},
						}
					})

					It("should run schedular successfully", shouldStartupSchedular)
				})

				Context("with GCS", func() {
					BeforeEach(func() {
						secret = f.SecretForGCSBackend()
						mongodb.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 20s",
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								GCS: &store.GCSSpec{
									Bucket: os.Getenv(GCS_BUCKET_NAME),
								},
							},
						}
					})

					It("should run schedular successfully", shouldStartupSchedular)

					Context("With Replica Set", func() {
						BeforeEach(func() {
							mongodb = f.MongoDBRS()
							mongodb.Spec.BackupSchedule = &api.BackupScheduleSpec{
								CronExpression: "@every 20s",
								Backend: store.Backend{
									StorageSecretName: secret.Name,
									Local: &store.LocalSpec{
										MountPath: "/repo",
										VolumeSource: core.VolumeSource{
											EmptyDir: &core.EmptyDirVolumeSource{},
										},
									},
								},
							}
						})
						It("should take Snapshot successfully", shouldStartupSchedular)
					})

					Context("With Sharding", func() {
						BeforeEach(func() {
							mongodb = f.MongoDBShard()
							mongodb.Spec.BackupSchedule = &api.BackupScheduleSpec{
								CronExpression: "@every 20s",
								Backend: store.Backend{
									StorageSecretName: secret.Name,
									Local: &store.LocalSpec{
										MountPath: "/repo",
										VolumeSource: core.VolumeSource{
											EmptyDir: &core.EmptyDirVolumeSource{},
										},
									},
								},
							}
						})
						It("should take Snapshot successfully", shouldStartupSchedular)
					})

				})
			})

			Context("With Update - with Local", func() {
				BeforeEach(func() {
					secret = f.SecretForLocalBackend()
				})

				var shouldScheduleWithUpdate = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Create Secret")
					err := f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					By("Update mongodb")
					_, err = f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 20s",
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Local: &store.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										EmptyDir: &core.EmptyDirVolumeSource{},
									},
								},
							},
						}

						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Count multiple Snapshot Object")
					f.EventuallySnapshotCount(mongodb.ObjectMeta).Should(matcher.MoreThan(3))

					By("Remove Backup Scheduler from MongoDB")
					_, err = f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.BackupSchedule = nil
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Verify multiple Succeeded Snapshot")
					f.EventuallyMultipleSnapshotFinishedProcessing(mongodb.ObjectMeta).Should(Succeed())

					deleteTestResource()
				}

				It("should run schedular successfully", shouldScheduleWithUpdate)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
					})
					It("should take Snapshot successfully", shouldScheduleWithUpdate)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
					})
					It("should take Snapshot successfully", shouldScheduleWithUpdate)
				})
			})

			Context("Re-Use DormantDatabase's scheduler", func() {
				BeforeEach(func() {
					secret = f.SecretForLocalBackend()
				})

				var shouldeReUseDormantDBcheduler = func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Create Secret")
					err := f.CreateSecret(secret)
					Expect(err).NotTo(HaveOccurred())

					By("Update mongodb")
					_, err = f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 20s",
							Backend: store.Backend{
								StorageSecretName: secret.Name,
								Local: &store.LocalSpec{
									MountPath: "/repo",
									VolumeSource: core.VolumeSource{
										EmptyDir: &core.EmptyDirVolumeSource{},
									},
								},
							},
						}
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Insert Document Inside DB")
					f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Count multiple Snapshot Object")
					f.EventuallySnapshotCount(mongodb.ObjectMeta).Should(matcher.MoreThan(3))

					By("Verify multiple Succeeded Snapshot")
					f.EventuallyMultipleSnapshotFinishedProcessing(mongodb.ObjectMeta).Should(Succeed())

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for mongodb to be paused")
					f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

					// Create MongoDB object again to resume it
					By("Create MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					By("Count multiple Snapshot Object")
					f.EventuallySnapshotCount(mongodb.ObjectMeta).Should(matcher.MoreThan(5))

					By("Remove Backup Scheduler from MongoDB")
					_, err = f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.BackupSchedule = nil
						return in
					})
					Expect(err).NotTo(HaveOccurred())

					By("Verify multiple Succeeded Snapshot")
					f.EventuallyMultipleSnapshotFinishedProcessing(mongodb.ObjectMeta).Should(Succeed())
				}

				It("should re-use scheduler successfully", shouldeReUseDormantDBcheduler)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
					})
					It("should take Snapshot successfully", shouldeReUseDormantDBcheduler)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
					})
					It("should take Snapshot successfully", shouldeReUseDormantDBcheduler)
				})
			})
		})

		Context("Termination Policy", func() {
			BeforeEach(func() {
				skipSnapshotDataChecking = false
				secret = f.SecretForS3Backend()
				snapshot.Spec.StorageSecretName = secret.Name
				snapshot.Spec.S3 = &store.S3Spec{
					Bucket: os.Getenv(S3_BUCKET_NAME),
				}
				snapshot.Spec.DatabaseName = mongodb.Name
			})

			AfterEach(func() {
				if snapshot != nil {
					By("Delete Existing snapshot")
					err := f.DeleteSnapshot(snapshot.ObjectMeta)
					if err != nil {
						if kerr.IsNotFound(err) {
							// MongoDB was not created. Hence, rest of cleanup is not necessary.
							return
						}
						Expect(err).NotTo(HaveOccurred())
					}
				}
			})

			var shouldRunWithSnapshot = func() {
				// Create and wait for running MongoDB
				createAndWaitForRunning()

				By("Insert Document Inside DB")
				f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

				By("Checking Inserted Document")
				f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

				By("Create Secret")
				err := f.CreateSecret(secret)
				Expect(err).NotTo(HaveOccurred())

				By("Create Snapshot")
				err = f.CreateSnapshot(snapshot)
				Expect(err).NotTo(HaveOccurred())

				By("Check for succeeded snapshot")
				f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSucceeded))

				if !skipSnapshotDataChecking {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
				}
			}

			Context("with TerminationDoNotTerminate", func() {
				BeforeEach(func() {
					skipSnapshotDataChecking = true
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
						in.Spec.TerminationPolicy = api.TerminationPolicyPause
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
					})
					It("should run successfully", shouldWorkDoNotTerminate)
				})

			})

			Context("with TerminationPolicyPause (default)", func() {
				var shouldRunWithTerminationPause = func() {
					shouldRunWithSnapshot()

					By("Delete mongodb")
					err = f.DeleteMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					// DormantDatabase.Status= paused, means mongodb object is deleted
					By("Wait for mongodb to be paused")
					f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

					By("Check for intact snapshot")
					_, err := f.GetSnapshot(snapshot.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					if !skipSnapshotDataChecking {
						By("Check for snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
					}

					// Create MongoDB object again to resume it
					By("Create (pause) MongoDB: " + mongodb.Name)
					err = f.CreateMongoDB(mongodb)
					Expect(err).NotTo(HaveOccurred())

					By("Wait for DormantDatabase to be deleted")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Wait for Running mongodb")
					f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

					mongodb, err = f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Checking Inserted Document")
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

				}

				It("should create dormantdatabase successfully", shouldRunWithTerminationPause)

				Context("with Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						snapshot.Spec.DatabaseName = mongodb.Name
					})

					It("should create dormantdatabase successfully", shouldRunWithTerminationPause)
				})

				Context("with Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						snapshot.Spec.DatabaseName = mongodb.Name
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

					By("Checking DormantDatabase is not created")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Check for deleted PVCs")
					f.EventuallyPVCCount(mongodb.ObjectMeta).Should(Equal(0))

					By("Check for intact Secrets")
					f.EventuallyDBSecretCount(mongodb.ObjectMeta).ShouldNot(Equal(0))

					By("Check for intact snapshot")
					_, err := f.GetSnapshot(snapshot.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					if !skipSnapshotDataChecking {
						By("Check for intact snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
					}

					By("Delete snapshot")
					err = f.DeleteSnapshot(snapshot.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					if !skipSnapshotDataChecking {
						By("Check for deleted snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
					}
				}

				It("should run with TerminationPolicyDelete", shouldRunWithTerminationDelete)

				Context("with Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyDelete
						snapshot.Spec.DatabaseName = mongodb.Name
					})
					It("should initialize database successfully", shouldRunWithTerminationDelete)
				})

				Context("with Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyDelete
						snapshot.Spec.DatabaseName = mongodb.Name
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

					By("Checking DormantDatabase is not created")
					f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

					By("Check for deleted PVCs")
					f.EventuallyPVCCount(mongodb.ObjectMeta).Should(Equal(0))

					By("Check for deleted Secrets")
					f.EventuallyDBSecretCount(mongodb.ObjectMeta).Should(Equal(0))

					By("Check for deleted Snapshots")
					f.EventuallySnapshotCount(snapshot.ObjectMeta).Should(Equal(0))

					if !skipSnapshotDataChecking {
						By("Check for deleted snapshot data")
						f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
					}
				}

				It("should run with TerminationPolicyWipeOut", shouldRunWithTerminationWipeOut)

				Context("with Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						snapshot.Spec.DatabaseName = mongodb.Name
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					It("should initialize database successfully", shouldRunWithTerminationWipeOut)
				})

				Context("with Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						snapshot.Spec.DatabaseName = mongodb.Name
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
					})
					It("should initialize database successfully", shouldRunWithTerminationWipeOut)
				})
			})
		})

		Context("Environment Variables", func() {

			Context("With allowed Envs", func() {
				BeforeEach(func() {
					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
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
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())
				}

				It("should initialize database specified by env", withAllowedEnvs)

				Context("With Replica Set", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBRS()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})
					It("should take Snapshot successfully", withAllowedEnvs)
				})

				Context("With Sharding", func() {
					BeforeEach(func() {
						mongodb = f.MongoDBShard()
						mongodb.Spec.Init = &api.InitSpec{
							ScriptSource: &api.ScriptSourceSpec{
								VolumeSource: core.VolumeSource{
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
					})
					It("should take Snapshot successfully", withAllowedEnvs)
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
					})
					It("should take Snapshot successfully", withForbiddenEnvs)
				})
			})

			Context("Update Envs", func() {
				BeforeEach(func() {
					mongodb.Spec.Init = &api.InitSpec{
						ScriptSource: &api.ScriptSourceSpec{
							VolumeSource: core.VolumeSource{
								GitRepo: &core.GitRepoVolumeSource{
									Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
									Directory:  ".",
								},
							},
						},
					}
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
					f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

					_, _, err = util.PatchMongoDB(f.ExtClient().KubedbV1alpha1(), mongodb, func(in *api.MongoDB) *api.MongoDB {
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
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
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
									GitRepo: &core.GitRepoVolumeSource{
										Repository: "https://github.com/kubedb/mongodb-init-scripts.git",
										Directory:  ".",
									},
								},
							},
						}
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
				f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())

				By("Checking Inserted Document")
				f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, framework.IsRepSet(mongodb), 1).Should(BeTrue())
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
					})

					It("should run successfully", shouldRunSuccessfully)
				})

				Context("With TerminationPolicyPause", func() {

					BeforeEach(func() {
						mongodb.Spec.StorageType = api.StorageTypeEphemeral
						mongodb.Spec.Storage = nil
						mongodb.Spec.TerminationPolicy = api.TerminationPolicyPause
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
