# Change Log

## [0.5.0](https://github.com/kubedb/mongodb/tree/0.5.0) (2019-05-06)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.4.0...0.5.0)

**Merged pull requests:**

- \[revendored\] Remove Resources field from shardTopology [\#154](https://github.com/kubedb/mongodb/pull/154) ([the-redback](https://github.com/the-redback))
- Revendor dependencies [\#153](https://github.com/kubedb/mongodb/pull/153) ([tamalsaha](https://github.com/tamalsaha))
- Use official Mongo GO Driver for testing MongoDB [\#152](https://github.com/kubedb/mongodb/pull/152) ([the-redback](https://github.com/the-redback))
- Fix PSP in Role for kubeDB upgrade [\#151](https://github.com/kubedb/mongodb/pull/151) ([iamrz1](https://github.com/iamrz1))
- Modify mutator validator names [\#150](https://github.com/kubedb/mongodb/pull/150) ([iamrz1](https://github.com/iamrz1))
- set resource requirements for bootstrap init container [\#149](https://github.com/kubedb/mongodb/pull/149) ([phin1x](https://github.com/phin1x))
- Add MongoDB Sharding support [\#148](https://github.com/kubedb/mongodb/pull/148) ([the-redback](https://github.com/the-redback))

## [0.4.0](https://github.com/kubedb/mongodb/tree/0.4.0) (2019-03-18)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.3.0...0.4.0)

**Merged pull requests:**

- Init container and DB psp in e2e test framework [\#147](https://github.com/kubedb/mongodb/pull/147) ([iamrz1](https://github.com/iamrz1))
- Don't inherit app.kubernetes.io labels from CRD into offshoots [\#146](https://github.com/kubedb/mongodb/pull/146) ([tamalsaha](https://github.com/tamalsaha))
- Support for init container  [\#145](https://github.com/kubedb/mongodb/pull/145) ([iamrz1](https://github.com/iamrz1))
- Add role label to stats service [\#144](https://github.com/kubedb/mongodb/pull/144) ([tamalsaha](https://github.com/tamalsaha))
- PSP support for MongoDB [\#143](https://github.com/kubedb/mongodb/pull/143) ([iamrz1](https://github.com/iamrz1))
- Update Kubernetes client libraries to 1.13.0 release [\#142](https://github.com/kubedb/mongodb/pull/142) ([tamalsaha](https://github.com/tamalsaha))

## [0.3.0](https://github.com/kubedb/mongodb/tree/0.3.0) (2019-02-19)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.2.0...0.3.0)

**Merged pull requests:**

- Revendor dependencies [\#141](https://github.com/kubedb/mongodb/pull/141) ([tamalsaha](https://github.com/tamalsaha))
- Revendor dependencies : Retry Failed Scheduler Snapshot [\#140](https://github.com/kubedb/mongodb/pull/140) ([the-redback](https://github.com/the-redback))
- Support for mongo 4.x [\#139](https://github.com/kubedb/mongodb/pull/139) ([the-redback](https://github.com/the-redback))
- Added ephemeral StorageType support [\#138](https://github.com/kubedb/mongodb/pull/138) ([the-redback](https://github.com/the-redback))
- Initial RBAC support: create and use K8s service account for MongoDB … [\#137](https://github.com/kubedb/mongodb/pull/137) ([maartenvandenbogaard](https://github.com/maartenvandenbogaard))
- Use PVC spec from snapshot if provided [\#135](https://github.com/kubedb/mongodb/pull/135) ([tamalsaha](https://github.com/tamalsaha))
- Revendored and updated tests for 'Prevent prefix matching of multiple snapshots' [\#134](https://github.com/kubedb/mongodb/pull/134) ([the-redback](https://github.com/the-redback))
- Update peer-finder binary [\#133](https://github.com/kubedb/mongodb/pull/133) ([the-redback](https://github.com/the-redback))
- Add certificate health checker [\#132](https://github.com/kubedb/mongodb/pull/132) ([tamalsaha](https://github.com/tamalsaha))
- Update E2E test: Env update is not restricted anymore [\#131](https://github.com/kubedb/mongodb/pull/131) ([the-redback](https://github.com/the-redback))
- Fix AppBinding [\#130](https://github.com/kubedb/mongodb/pull/130) ([tamalsaha](https://github.com/tamalsaha))

## [0.2.0](https://github.com/kubedb/mongodb/tree/0.2.0) (2018-12-17)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.2.0-rc.2...0.2.0)

**Fixed bugs:**

- Fix panic for mongodb probe [\#125](https://github.com/kubedb/mongodb/pull/125) ([the-redback](https://github.com/the-redback))

**Merged pull requests:**

- Reuse event recorder [\#129](https://github.com/kubedb/mongodb/pull/129) ([tamalsaha](https://github.com/tamalsaha))
- Revendor dependencies [\#128](https://github.com/kubedb/mongodb/pull/128) ([tamalsaha](https://github.com/tamalsaha))
- OSM binay upgraded & E2E tests for multiple collecion [\#127](https://github.com/kubedb/mongodb/pull/127) ([the-redback](https://github.com/the-redback))
- Test for faulty snapshot [\#126](https://github.com/kubedb/mongodb/pull/126) ([the-redback](https://github.com/the-redback))

## [0.2.0-rc.2](https://github.com/kubedb/mongodb/tree/0.2.0-rc.2) (2018-12-06)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.2.0-rc.1...0.2.0-rc.2)

**Merged pull requests:**

- Upgrade database secret keys  [\#124](https://github.com/kubedb/mongodb/pull/124) ([the-redback](https://github.com/the-redback))
- Ignore mutation of fields to default values during update [\#123](https://github.com/kubedb/mongodb/pull/123) ([tamalsaha](https://github.com/tamalsaha))
- Support configuration options for exporter sidecar [\#122](https://github.com/kubedb/mongodb/pull/122) ([tamalsaha](https://github.com/tamalsaha))
- Use flags.DumpAll [\#121](https://github.com/kubedb/mongodb/pull/121) ([tamalsaha](https://github.com/tamalsaha))

## [0.2.0-rc.1](https://github.com/kubedb/mongodb/tree/0.2.0-rc.1) (2018-12-02)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.2.0-rc.0...0.2.0-rc.1)

**Merged pull requests:**

- Apply cleanup [\#120](https://github.com/kubedb/mongodb/pull/120) ([tamalsaha](https://github.com/tamalsaha))
- Set periodic analytics [\#119](https://github.com/kubedb/mongodb/pull/119) ([tamalsaha](https://github.com/tamalsaha))
- Introduce AppBinding support [\#118](https://github.com/kubedb/mongodb/pull/118) ([the-redback](https://github.com/the-redback))
- Fix analytics [\#117](https://github.com/kubedb/mongodb/pull/117) ([the-redback](https://github.com/the-redback))
- Fix Mongo liveness/readiness probes overwrite podTemplate configuration [\#116](https://github.com/kubedb/mongodb/pull/116) ([the-redback](https://github.com/the-redback))
- Error out from cron job for deprecated dbversion [\#115](https://github.com/kubedb/mongodb/pull/115) ([the-redback](https://github.com/the-redback))
- Fix operator startup in minikube [\#114](https://github.com/kubedb/mongodb/pull/114) ([the-redback](https://github.com/the-redback))
- Removed WaitUntilDigitalOceanReady from e2e tests [\#113](https://github.com/kubedb/mongodb/pull/113) ([the-redback](https://github.com/the-redback))
- Add CRDs without observation when operator starts [\#112](https://github.com/kubedb/mongodb/pull/112) ([the-redback](https://github.com/the-redback))
- Fix DNS for mongodb hosts [\#111](https://github.com/kubedb/mongodb/pull/111) ([the-redback](https://github.com/the-redback))

## [0.2.0-rc.0](https://github.com/kubedb/mongodb/tree/0.2.0-rc.0) (2018-10-15)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.2.0-beta.1...0.2.0-rc.0)

**Merged pull requests:**

- Support providing resources for monitoring container [\#110](https://github.com/kubedb/mongodb/pull/110) ([tamalsaha](https://github.com/tamalsaha))
- Update kubernetes client libraries to 1.12.0 [\#109](https://github.com/kubedb/mongodb/pull/109) ([tamalsaha](https://github.com/tamalsaha))
- Add validation webhook xray [\#108](https://github.com/kubedb/mongodb/pull/108) ([tamalsaha](https://github.com/tamalsaha))
- Various Fixes [\#107](https://github.com/kubedb/mongodb/pull/107) ([hossainemruz](https://github.com/hossainemruz))
- Fix host for mongodb backup and restore jobs [\#106](https://github.com/kubedb/mongodb/pull/106) ([the-redback](https://github.com/the-redback))
- Use dynamic username for mongodb backup and restore [\#105](https://github.com/kubedb/mongodb/pull/105) ([the-redback](https://github.com/the-redback))
- Merge ports from service template [\#103](https://github.com/kubedb/mongodb/pull/103) ([tamalsaha](https://github.com/tamalsaha))
- Replace doNotPause with TerminationPolicy = DoNotTerminate [\#102](https://github.com/kubedb/mongodb/pull/102) ([tamalsaha](https://github.com/tamalsaha))
- Pass resources to NamespaceValidator [\#101](https://github.com/kubedb/mongodb/pull/101) ([tamalsaha](https://github.com/tamalsaha))
- Various fixes [\#100](https://github.com/kubedb/mongodb/pull/100) ([tamalsaha](https://github.com/tamalsaha))
- Support Livecycle hook and container probes [\#99](https://github.com/kubedb/mongodb/pull/99) ([tamalsaha](https://github.com/tamalsaha))
- Check if Kubernetes version is supported before running operator [\#98](https://github.com/kubedb/mongodb/pull/98) ([tamalsaha](https://github.com/tamalsaha))
- Update package alias [\#97](https://github.com/kubedb/mongodb/pull/97) ([tamalsaha](https://github.com/tamalsaha))

## [0.2.0-beta.1](https://github.com/kubedb/mongodb/tree/0.2.0-beta.1) (2018-09-30)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.2.0-beta.0...0.2.0-beta.1)

**Merged pull requests:**

- Revendor api [\#96](https://github.com/kubedb/mongodb/pull/96) ([tamalsaha](https://github.com/tamalsaha))
- Fix tests [\#95](https://github.com/kubedb/mongodb/pull/95) ([tamalsaha](https://github.com/tamalsaha))
- Revendor api for catalog apigroup [\#94](https://github.com/kubedb/mongodb/pull/94) ([tamalsaha](https://github.com/tamalsaha))
- Fix: Restrict user from updating spec.storageType [\#93](https://github.com/kubedb/mongodb/pull/93) ([the-redback](https://github.com/the-redback))
- Use --pull flag with docker build \(\#20\) [\#92](https://github.com/kubedb/mongodb/pull/92) ([tamalsaha](https://github.com/tamalsaha))

## [0.2.0-beta.0](https://github.com/kubedb/mongodb/tree/0.2.0-beta.0) (2018-09-20)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.1.0...0.2.0-beta.0)

**Fixed bugs:**

- Update status.ObservedGeneration for failure phase [\#72](https://github.com/kubedb/mongodb/pull/72) ([the-redback](https://github.com/the-redback))

**Merged pull requests:**

- Show deprecated column for mongodbversions [\#91](https://github.com/kubedb/mongodb/pull/91) ([hossainemruz](https://github.com/hossainemruz))
- Pass extra args to tools.sh [\#90](https://github.com/kubedb/mongodb/pull/90) ([the-redback](https://github.com/the-redback))
-  Support Termination Policy & Stop working for deprecated \*Versions [\#89](https://github.com/kubedb/mongodb/pull/89) ([the-redback](https://github.com/the-redback))
- Revendor k8s.io/apiserver [\#88](https://github.com/kubedb/mongodb/pull/88) ([tamalsaha](https://github.com/tamalsaha))
- Revendor kubernetes-1.11.3 [\#87](https://github.com/kubedb/mongodb/pull/87) ([tamalsaha](https://github.com/tamalsaha))
- Don't try to wipe out Snapshot data for Local backend [\#86](https://github.com/kubedb/mongodb/pull/86) ([hossainemruz](https://github.com/hossainemruz))
- Support UpdateStrategy [\#85](https://github.com/kubedb/mongodb/pull/85) ([tamalsaha](https://github.com/tamalsaha))
- Add TerminationPolicy for databases [\#84](https://github.com/kubedb/mongodb/pull/84) ([tamalsaha](https://github.com/tamalsaha))
- Revendor api [\#83](https://github.com/kubedb/mongodb/pull/83) ([tamalsaha](https://github.com/tamalsaha))
- Fix log formatting [\#82](https://github.com/kubedb/mongodb/pull/82) ([tamalsaha](https://github.com/tamalsaha))
- Use IntHash as status.observedGeneration [\#81](https://github.com/kubedb/mongodb/pull/81) ([tamalsaha](https://github.com/tamalsaha))
- fix github status [\#80](https://github.com/kubedb/mongodb/pull/80) ([tahsinrahman](https://github.com/tahsinrahman))
- update pipeline [\#79](https://github.com/kubedb/mongodb/pull/79) ([tahsinrahman](https://github.com/tahsinrahman))
- update pipeline [\#78](https://github.com/kubedb/mongodb/pull/78) ([tahsinrahman](https://github.com/tahsinrahman))
- maintain exporter docker image latest tag from master branch [\#76](https://github.com/kubedb/mongodb/pull/76) ([the-redback](https://github.com/the-redback))
- Use k8s.io/apiserver from pharmer [\#75](https://github.com/kubedb/mongodb/pull/75) ([the-redback](https://github.com/the-redback))
-  Use officially suggested exporter image [\#74](https://github.com/kubedb/mongodb/pull/74) ([the-redback](https://github.com/the-redback))
- Migrate MongoDB [\#73](https://github.com/kubedb/mongodb/pull/73) ([tamalsaha](https://github.com/tamalsaha))
- Keep track of ObservedGenerationHash [\#71](https://github.com/kubedb/mongodb/pull/71) ([tamalsaha](https://github.com/tamalsaha))
- Use NewObservableHandler [\#70](https://github.com/kubedb/mongodb/pull/70) ([tamalsaha](https://github.com/tamalsaha))
- Fix uninstall for concourse [\#69](https://github.com/kubedb/mongodb/pull/69) ([tahsinrahman](https://github.com/tahsinrahman))
- Support passing args via PodTemplate [\#68](https://github.com/kubedb/mongodb/pull/68) ([tamalsaha](https://github.com/tamalsaha))
- Introduce storageType : ephemeral [\#67](https://github.com/kubedb/mongodb/pull/67) ([tamalsaha](https://github.com/tamalsaha))
- Revendor api [\#66](https://github.com/kubedb/mongodb/pull/66) ([tamalsaha](https://github.com/tamalsaha))
- Add support for running tests on cncf cluster [\#65](https://github.com/kubedb/mongodb/pull/65) ([tahsinrahman](https://github.com/tahsinrahman))
- Revendor api [\#64](https://github.com/kubedb/mongodb/pull/64) ([tamalsaha](https://github.com/tamalsaha))
- Revendor apimachinery [\#63](https://github.com/kubedb/mongodb/pull/63) ([tamalsaha](https://github.com/tamalsaha))
- Use ObservedGeneration in Status to keep track of last generation observed [\#62](https://github.com/kubedb/mongodb/pull/62) ([the-redback](https://github.com/the-redback))
- Separate StatsService for monitoring [\#61](https://github.com/kubedb/mongodb/pull/61) ([the-redback](https://github.com/the-redback))
- Use MongoDBVersion for Mongodb images [\#60](https://github.com/kubedb/mongodb/pull/60) ([the-redback](https://github.com/the-redback))
- Use updated crd spec [\#59](https://github.com/kubedb/mongodb/pull/59) ([tamalsaha](https://github.com/tamalsaha))
- Rename OffshootLabels to OffshootSelectors [\#58](https://github.com/kubedb/mongodb/pull/58) ([tamalsaha](https://github.com/tamalsaha))
- Revendor api [\#57](https://github.com/kubedb/mongodb/pull/57) ([tamalsaha](https://github.com/tamalsaha))
- Use kmodules monitoring and objectstore api [\#56](https://github.com/kubedb/mongodb/pull/56) ([tamalsaha](https://github.com/tamalsaha))
- Refactor concourse scripts [\#55](https://github.com/kubedb/mongodb/pull/55) ([tahsinrahman](https://github.com/tahsinrahman))
- Fix command `./hack/make.py test e2e` [\#54](https://github.com/kubedb/mongodb/pull/54) ([the-redback](https://github.com/the-redback))
- Set generated binary name to mg-operator [\#53](https://github.com/kubedb/mongodb/pull/53) ([tamalsaha](https://github.com/tamalsaha))
- Don't add admission/v1beta1 group as a prioritized version [\#52](https://github.com/kubedb/mongodb/pull/52) ([tamalsaha](https://github.com/tamalsaha))
- Enable status subresource for crds [\#51](https://github.com/kubedb/mongodb/pull/51) ([tamalsaha](https://github.com/tamalsaha))
- Update client-go to v8.0.0 [\#50](https://github.com/kubedb/mongodb/pull/50) ([tamalsaha](https://github.com/tamalsaha))
- Format shell script [\#49](https://github.com/kubedb/mongodb/pull/49) ([tamalsaha](https://github.com/tamalsaha))
- Mongodb Clustering - replicaset && config file addition [\#48](https://github.com/kubedb/mongodb/pull/48) ([the-redback](https://github.com/the-redback))
-  Updated osm version to 0.7.1 [\#47](https://github.com/kubedb/mongodb/pull/47) ([the-redback](https://github.com/the-redback))
- Support ENV variables in CRDs [\#46](https://github.com/kubedb/mongodb/pull/46) ([hossainemruz](https://github.com/hossainemruz))

## [0.1.0](https://github.com/kubedb/mongodb/tree/0.1.0) (2018-06-12)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.1.0-rc.0...0.1.0)

**Merged pull requests:**

- Fixed missing error return [\#44](https://github.com/kubedb/mongodb/pull/44) ([the-redback](https://github.com/the-redback))
- Revendor dependencies [\#43](https://github.com/kubedb/mongodb/pull/43) ([tamalsaha](https://github.com/tamalsaha))
- Add changelog [\#42](https://github.com/kubedb/mongodb/pull/42) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0-rc.0](https://github.com/kubedb/mongodb/tree/0.1.0-rc.0) (2018-05-28)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.1.0-beta.2...0.1.0-rc.0)

**Merged pull requests:**

- Concourse [\#41](https://github.com/kubedb/mongodb/pull/41) ([tahsinrahman](https://github.com/tahsinrahman))
- Fixed kubeconfig plugin for Cloud Providers && Storage is required for MongoDB [\#40](https://github.com/kubedb/mongodb/pull/40) ([the-redback](https://github.com/the-redback))
-  Do not delete Admission configs in E2E tests, if operator is self-hosted [\#39](https://github.com/kubedb/mongodb/pull/39) ([the-redback](https://github.com/the-redback))
-  Refactored E2E testing to support E2E testing with admission webhook in cloud [\#38](https://github.com/kubedb/mongodb/pull/38) ([the-redback](https://github.com/the-redback))
- Skip delete requests for empty resources [\#37](https://github.com/kubedb/mongodb/pull/37) ([the-redback](https://github.com/the-redback))
- Don't panic if admission options is nil [\#36](https://github.com/kubedb/mongodb/pull/36) ([tamalsaha](https://github.com/tamalsaha))
- Disable admission controllers for webhook server [\#35](https://github.com/kubedb/mongodb/pull/35) ([tamalsaha](https://github.com/tamalsaha))
-  Separate ApiGroup for Mutating and Validating webhook && upgraded osm to 0.7.0 [\#34](https://github.com/kubedb/mongodb/pull/34) ([the-redback](https://github.com/the-redback))
- Update client-go to 7.0.0 [\#33](https://github.com/kubedb/mongodb/pull/33) ([tamalsaha](https://github.com/tamalsaha))
-  Added support for one watcher and N-eventHandler for Snapshot, DormantDB and Job [\#32](https://github.com/kubedb/mongodb/pull/32) ([the-redback](https://github.com/the-redback))
- Use metrics from kube apiserver [\#31](https://github.com/kubedb/mongodb/pull/31) ([tamalsaha](https://github.com/tamalsaha))
- Fix e2e tests for rbac enabled cluster [\#30](https://github.com/kubedb/mongodb/pull/30) ([the-redback](https://github.com/the-redback))
- Bundle webhook server [\#29](https://github.com/kubedb/mongodb/pull/29) ([tamalsaha](https://github.com/tamalsaha))
-  Moved MongoDB Admission Controller packages into mongodb [\#28](https://github.com/kubedb/mongodb/pull/28) ([the-redback](https://github.com/the-redback))
- Add travis yaml [\#27](https://github.com/kubedb/mongodb/pull/27) ([tahsinrahman](https://github.com/tahsinrahman))
- Refactored MongoDB Controller to support mutating webhook [\#25](https://github.com/kubedb/mongodb/pull/25) ([the-redback](https://github.com/the-redback))

## [0.1.0-beta.2](https://github.com/kubedb/mongodb/tree/0.1.0-beta.2) (2018-02-27)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.1.0-beta.1...0.1.0-beta.2)

**Merged pull requests:**

- Use AppsV1\(\) to get StatefulSets [\#24](https://github.com/kubedb/mongodb/pull/24) ([the-redback](https://github.com/the-redback))
- Migrating to apps/v1 [\#23](https://github.com/kubedb/mongodb/pull/23) ([the-redback](https://github.com/the-redback))
- update validation [\#22](https://github.com/kubedb/mongodb/pull/22) ([aerokite](https://github.com/aerokite))
- Fix dormantDB matching: pass same type to Equal method [\#21](https://github.com/kubedb/mongodb/pull/21) ([the-redback](https://github.com/the-redback))
- Use official code generator scripts [\#20](https://github.com/kubedb/mongodb/pull/20) ([tamalsaha](https://github.com/tamalsaha))
- Fixed dormantdb matching & Raised trottling time & Fixed MongoDB version Checking [\#19](https://github.com/kubedb/mongodb/pull/19) ([the-redback](https://github.com/the-redback))
-  Set Env from Secret ref & Fixed database connection in test [\#18](https://github.com/kubedb/mongodb/pull/18) ([the-redback](https://github.com/the-redback))

## [0.1.0-beta.1](https://github.com/kubedb/mongodb/tree/0.1.0-beta.1) (2018-01-29)
[Full Changelog](https://github.com/kubedb/mongodb/compare/0.1.0-beta.0...0.1.0-beta.1)

**Merged pull requests:**

- converted to k8s 1.9 & Improved InitSpec in DormantDB &  Added support for Job watcher [\#16](https://github.com/kubedb/mongodb/pull/16) ([the-redback](https://github.com/the-redback))
- Fix analytics, logger and send Exporter Secret as mounted path [\#15](https://github.com/kubedb/mongodb/pull/15) ([the-redback](https://github.com/the-redback))
- Simplify DB auth secret [\#14](https://github.com/kubedb/mongodb/pull/14) ([tamalsaha](https://github.com/tamalsaha))
- Review db docker images [\#13](https://github.com/kubedb/mongodb/pull/13) ([tamalsaha](https://github.com/tamalsaha))

## [0.1.0-beta.0](https://github.com/kubedb/mongodb/tree/0.1.0-beta.0) (2018-01-07)
**Merged pull requests:**

- Fix Analytics and pass client-id as ENV to Snapshot Job [\#12](https://github.com/kubedb/mongodb/pull/12) ([the-redback](https://github.com/the-redback))
- Add docker-registry and WorkQueue [\#10](https://github.com/kubedb/mongodb/pull/10) ([the-redback](https://github.com/the-redback))
- Use client id for analytics [\#9](https://github.com/kubedb/mongodb/pull/9) ([tamalsaha](https://github.com/tamalsaha))
- Fix CRD registration [\#8](https://github.com/kubedb/mongodb/pull/8) ([the-redback](https://github.com/the-redback))
- Update pkg paths to kubedb org [\#7](https://github.com/kubedb/mongodb/pull/7) ([tamalsaha](https://github.com/tamalsaha))
- Assign default Prometheus Monitoring Port [\#6](https://github.com/kubedb/mongodb/pull/6) ([the-redback](https://github.com/the-redback))
- Add Snapshot Schedule [\#5](https://github.com/kubedb/mongodb/pull/5) ([the-redback](https://github.com/the-redback))
- Add Snapshot Backup and Restore [\#4](https://github.com/kubedb/mongodb/pull/4) ([the-redback](https://github.com/the-redback))
- Add mongodb-util docker image [\#3](https://github.com/kubedb/mongodb/pull/3) ([the-redback](https://github.com/the-redback))
- Initial mongo [\#2](https://github.com/kubedb/mongodb/pull/2) ([the-redback](https://github.com/the-redback))
- Add MongoDB controller skeleton [\#1](https://github.com/kubedb/mongodb/pull/1) ([tamalsaha](https://github.com/tamalsaha))



\* *This Change Log was automatically generated by [github_changelog_generator](https://github.com/skywinder/Github-Changelog-Generator)*