package e2e_test

import (
	"fmt"
	"os"

	"github.com/appscode/go/types"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/mongodb/test/e2e/framework"
	"github.com/kubedb/mongodb/test/e2e/matcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	S3_BUCKET_NAME       = "S3_BUCKET_NAME"
	GCS_BUCKET_NAME      = "GCS_BUCKET_NAME"
	AZURE_CONTAINER_NAME = "AZURE_CONTAINER_NAME"
	SWIFT_CONTAINER_NAME = "SWIFT_CONTAINER_NAME"
)

var _ = Describe("MongoDB", func() {
	var (
		err         error
		f           *framework.Invocation
		mongodb     *api.MongoDB
		snapshot    *api.Snapshot
		secret      *core.Secret
		skipMessage string
	)

	BeforeEach(func() {
		f = root.Invoke()
		mongodb = f.MongoDB()
		snapshot = f.Snapshot()
		skipMessage = ""
	})

	var createAndWaitForRunning = func() {
		By("Create MongoDB: " + mongodb.Name)
		err = f.CreateMongoDB(mongodb)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running mongodb")
		f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())
	}

	var deleteTestResource = func() {
		By("Delete mongodb")
		err = f.DeleteMongoDB(mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb to be paused")
		f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

		By("WipeOut mongodb")
		_, err := f.PatchDormantDatabase(mongodb.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
			in.Spec.WipeOut = true
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb to be wipedOut")
		f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HaveWipedOut())

		err = f.DeleteDormantDatabase(mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())
	}

	var shouldSuccessfullyRunning = func() {
		if skipMessage != "" {
			Skip(skipMessage)
		}

		// Create MongoDB
		createAndWaitForRunning()

		// Delete test resource
		deleteTestResource()
	}

	Describe("Test", func() {

		Context("General", func() {

			Context("-", func() {
				It("should run successfully", shouldSuccessfullyRunning)
			})

			Context("With PVC", func() {
				BeforeEach(func() {
					if f.StorageClass == "" {
						skipMessage = "Missing StorageClassName. Provide as flag to test this."
					}
					mongodb.Spec.Storage = &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse("100Mi"),
							},
						},
						StorageClassName: types.StringP(f.StorageClass),
					}
				})
				It("should run successfully", shouldSuccessfullyRunning)
			})
		})

		XContext("DoNotPause", func() {
			BeforeEach(func() {
				mongodb.Spec.DoNotPause = true
			})

			It("should work successfully", func() {
				// Create and wait for running MongoDB
				createAndWaitForRunning()

				By("Delete mongodb")
				err = f.DeleteMongoDB(mongodb.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				By("MongoDB is not paused. Check for mongodb")
				f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeTrue())

				By("Check for Running mongodb")
				f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

				By("Update mongodb to set DoNotPause=false")
				f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
					in.Spec.DoNotPause = false
					return in
				})

				// Delete test resource
				deleteTestResource()
			})
		})

		Context("Snapshot", func() {
			var skipDataCheck bool

			AfterEach(func() {
				f.DeleteSecret(secret.ObjectMeta)
			})

			BeforeEach(func() {
				skipDataCheck = false
				snapshot.Spec.DatabaseName = mongodb.Name
			})

			var shouldTakeSnapshot = func() {
				// Create and wait for running MongoDB
				createAndWaitForRunning()

				By("Create Secret")
				f.CreateSecret(secret)

				By("Create Snapshot")
				f.CreateSnapshot(snapshot)

				By("Check for Successed snapshot")
				f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSuccessed))

				if !skipDataCheck {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())
				}

				// Delete test resource
				deleteTestResource()

				if !skipDataCheck {
					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeFalse())
				}
			}

			Context("In Local", func() {
				BeforeEach(func() {
					skipDataCheck = true
					secret = f.SecretForLocalBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Local = &api.LocalSpec{
						MountPath: "/repo",
						VolumeSource: core.VolumeSource{
							EmptyDir: &core.EmptyDirVolumeSource{},
						},
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				// Additional
				Context("With PVC", func() {
					BeforeEach(func() {
						// set f.storage from cli flag. Example:
						// ginkgo test/e2e/ -- -storageclass="standard"
						if f.StorageClass == "" {
							skipMessage = "Missing StorageClassName. Provide as flag to test this."
						}
						mongodb.Spec.Storage = &core.PersistentVolumeClaimSpec{
							Resources: core.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse("100Mi"),
								},
							},
							StorageClassName: types.StringP(f.StorageClass),
						}
					})
					It("should run successfully", shouldTakeSnapshot)
				})
			})

			Context("In S3", func() {
				BeforeEach(func() {
					secret = f.SecretForS3Backend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.S3 = &api.S3Spec{
						Bucket: os.Getenv(S3_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In GCS", func() {
				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &api.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)

				Context("With Init", func() {
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

					It("should resume DormantDatabase successfully", shouldTakeSnapshot)
				})
			})

			Context("In Azure", func() {
				BeforeEach(func() {
					secret = f.SecretForAzureBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Azure = &api.AzureSpec{
						Container: os.Getenv(AZURE_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
			})

			Context("In Swift", func() {
				BeforeEach(func() {
					secret = f.SecretForSwiftBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.Swift = &api.SwiftSpec{
						Container: os.Getenv(SWIFT_CONTAINER_NAME),
					}
				})

				It("should take Snapshot successfully", shouldTakeSnapshot)
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

				It("should run successfully", shouldSuccessfullyRunning)

			})

			Context("With Snapshot", func() {
				AfterEach(func() {
					f.DeleteSecret(secret.ObjectMeta)
				})

				BeforeEach(func() {
					secret = f.SecretForGCSBackend()
					snapshot.Spec.StorageSecretName = secret.Name
					snapshot.Spec.GCS = &api.GCSSpec{
						Bucket: os.Getenv(GCS_BUCKET_NAME),
					}
					snapshot.Spec.DatabaseName = mongodb.Name
				})

				It("should run successfully", func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Create Secret")
					f.CreateSecret(secret)

					By("Create Snapshot")
					f.CreateSnapshot(snapshot)

					By("Check for Successed snapshot")
					f.EventuallySnapshotPhase(snapshot.ObjectMeta).Should(Equal(api.SnapshotPhaseSuccessed))

					By("Check for snapshot data")
					f.EventuallySnapshotDataFound(snapshot).Should(BeTrue())

					oldMongoDB, err := f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					By("Create mongodb from snapshot")
					mongodb = f.MongoDB()
					mongodb.Spec.Init = &api.InitSpec{
						SnapshotSource: &api.SnapshotSourceSpec{
							Namespace: snapshot.Namespace,
							Name:      snapshot.Name,
						},
					}

					// Create and wait for running MongoDB
					createAndWaitForRunning()

					// Delete test resource
					deleteTestResource()
					mongodb = oldMongoDB
					// Delete test resource
					deleteTestResource()
				})
			})

		})

		Context("Resume", func() {
			var usedInitSpec bool
			BeforeEach(func() {
				usedInitSpec = false
			})

			var shouldResumeSuccessfully = func() {
				// Create and wait for running MongoDB
				createAndWaitForRunning()

				By("Delete mongodb")
				f.DeleteMongoDB(mongodb.ObjectMeta)

				By("Wait for mongodb to be paused")
				f.EventuallyDormantDatabaseStatus(mongodb.ObjectMeta).Should(matcher.HavePaused())

				_, err = f.PatchDormantDatabase(mongodb.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
					in.Spec.Resume = true
					return in
				})
				Expect(err).NotTo(HaveOccurred())

				By("Wait for DormantDatabase to be deleted")
				f.EventuallyDormantDatabase(mongodb.ObjectMeta).Should(BeFalse())

				By("Wait for Running mongodb")
				f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

				mongodb, err = f.GetMongoDB(mongodb.ObjectMeta)
				Expect(err).NotTo(HaveOccurred())

				if usedInitSpec {
					Expect(mongodb.Spec.Init).Should(BeNil())
					Expect(mongodb.Annotations[api.GenericInitSpec]).ShouldNot(BeEmpty())
				}

				// Delete test resource
				deleteTestResource()
			}

			Context("-", func() {
				It("should resume DormantDatabase successfully", shouldResumeSuccessfully)
			})

			Context("With Init", func() {
				BeforeEach(func() {
					usedInitSpec = true
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

				It("should resume DormantDatabase successfully", shouldResumeSuccessfully)
			})

			Context("With original MongoDB", func() {
				It("should resume DormantDatabase successfully", func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Delete mongodb")
					f.DeleteMongoDB(mongodb.ObjectMeta)

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

					_, err = f.GetMongoDB(mongodb.ObjectMeta)
					Expect(err).NotTo(HaveOccurred())

					if usedInitSpec {
						Expect(mongodb.Spec.Init).Should(BeNil())
						Expect(mongodb.Annotations[api.GenericInitSpec]).ShouldNot(BeEmpty())
					}

					// Delete test resource
					deleteTestResource()
				})

				Context("Multiple times with init", func() {
					BeforeEach(func() {
						usedInitSpec = true
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
						if f.StorageClass == "" {
							skipMessage = "Missing StorageClassName. Provide as flag to test this."
						}
						mongodb.Spec.Storage = &core.PersistentVolumeClaimSpec{
							Resources: core.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse("50Mi"),
								},
							},
							StorageClassName: types.StringP(f.StorageClass),
						}
					})

					It("should resume DormantDatabase successfully", func() {
						// Create and wait for running MongoDB
						createAndWaitForRunning()

						for i := 0; i < 3; i++ {
							By(fmt.Sprintf("%v-th", i+1) + " time running.")
							By("Delete mongodb")
							f.DeleteMongoDB(mongodb.ObjectMeta)

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
						}

						// Delete test resource
						deleteTestResource()
					})
				})
			})
		})

		Context("SnapshotScheduler", func() {
			AfterEach(func() {
				f.DeleteSecret(secret.ObjectMeta)
			})

			Context("With Startup", func() {

				var shouldStartupSchedular = func() {
					By("Create Secret")
					f.CreateSecret(secret)

					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Count multiple Snapshot")
					f.EventuallySnapshotCount(mongodb.ObjectMeta).Should(matcher.MoreThan(3))

					deleteTestResource()
				}

				Context("with local", func() {
					BeforeEach(func() {
						secret = f.SecretForLocalBackend()
						mongodb.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								Local: &api.LocalSpec{
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
				Context("with GCS and PVC", func() {
					BeforeEach(func() {
						secret = f.SecretForGCSBackend()
						mongodb.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								GCS: &api.GCSSpec{
									Bucket: os.Getenv(GCS_BUCKET_NAME),
								},
							},
						}
						if f.StorageClass == "" {
							skipMessage = "Missing StorageClassName. Provide as flag to test this."
						}
						mongodb.Spec.Storage = &core.PersistentVolumeClaimSpec{
							Resources: core.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse("50Mi"),
								},
							},
							StorageClassName: types.StringP(f.StorageClass),
						}
					})

					It("should run schedular successfully", shouldStartupSchedular)
				})
			})

			Context("With Update", func() {
				BeforeEach(func() {
					secret = f.SecretForLocalBackend()
				})
				It("should run schedular successfully", func() {
					// Create and wait for running MongoDB
					createAndWaitForRunning()

					By("Create Secret")
					f.CreateSecret(secret)

					By("Update mongodb")
					_, err = f.PatchMongoDB(mongodb.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
						in.Spec.BackupSchedule = &api.BackupScheduleSpec{
							CronExpression: "@every 1m",
							SnapshotStorageSpec: api.SnapshotStorageSpec{
								StorageSecretName: secret.Name,
								Local: &api.LocalSpec{
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

					By("Count multiple Snapshot")
					f.EventuallySnapshotCount(mongodb.ObjectMeta).Should(matcher.MoreThan(3))

					deleteTestResource()
				})
			})
		})
	})
})
