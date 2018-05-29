# Change Log

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
- update docker image validation [\#11](https://github.com/kubedb/mongodb/pull/11) ([aerokite](https://github.com/aerokite))
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