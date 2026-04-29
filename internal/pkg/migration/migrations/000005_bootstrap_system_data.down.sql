DELETE FROM `casbin_rule`
WHERE (`ptype`, `v0`, `v1`, `v2`, `v3`) IN (('p', 'role:super_admin', 'platform', '*', '.*'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:users',
                                             'read|search|create|update|deactivate|block|link_external_identity'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:profiles',
                                             'read|list|search|create|update'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:profile-links',
                                             'read|list|grant|update_relation|revoke|bulk_revoke|import'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:roles',
                                             'create|read|update|delete|list'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:assignments',
                                             'grant|revoke|delete|read'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:policies', 'read|write|delete'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:resources',
                                             'create|read|update|delete|list|validate_action'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:check', 'check'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:accounts',
                                             'read|update|enable|disable'),
                                            ('p', 'role:user', 'fangcun', 'iam:profile', 'read'),
                                            ('p', 'role:user', 'fangcun', 'iam:profile', 'update'),
                                            ('p', 'role:qs:admin', '1', 'qs:*', '.*'),
                                            ('p', 'role:qs:content_manager', '1', 'qs:questionnaires',
                                             'create|read|list|update|delete|publish|unpublish|archive|statistics'),
                                            ('p', 'role:qs:content_manager', '1', 'qs:scales',
                                             'create|read|list|update|delete|publish|unpublish|archive'),
                                            ('p', 'role:qs:evaluator', '1', 'qs:answersheets',
                                             'read|list|statistics'),
                                            ('p', 'role:qs:evaluator', '1', 'qs:assessments',
                                             'read|list|retry|batch_evaluate|statistics'),
                                            ('p', 'role:qs:evaluator', '1', 'qs:reports', 'read|list'),
                                            ('p', 'role:qs:evaluator', '1', 'qs:testees',
                                             'read|list|analyze|statistics'),
                                            ('p', 'role:qs:staff', '1', 'qs:testees', 'read|list'),
                                            ('p', 'role:qs:evaluation_plan_manager', '1', 'qs:evaluation_plans',
                                             'create|read|list|update|pause|resume|cancel|enroll|terminate|statistics'),
                                            ('p', 'role:qs:evaluation_plan_manager', '1',
                                             'qs:evaluation_plan_tasks',
                                             'schedule|read|list|open|complete|expire|cancel'));

DELETE FROM `casbin_rule`
WHERE `ptype` = 'g'
  AND (`v0`, `v1`, `v2`) IN (('role:tenant_admin', 'role:user', 'fangcun'),
                             ('role:qs:admin', 'role:qs:content_manager', '1'),
                             ('role:qs:admin', 'role:qs:evaluator', '1'),
                             ('role:qs:admin', 'role:qs:evaluation_plan_manager', '1'),
                             ('role:qs:evaluator', 'role:qs:staff', '1'),
                             ('role:qs:evaluation_plan_manager', 'role:qs:staff', '1'),
                             ('user:10001', 'role:super_admin', 'platform'),
                             ('user:10001', 'role:tenant_admin', 'fangcun'),
                             ('user:10001', 'role:qs:admin', '1'),
                             ('user:110001', 'role:tenant_admin', 'fangcun'),
                             ('user:110001', 'role:qs:admin', '1'),
                             ('user:110002', 'role:qs:content_manager', '1'));

DELETE FROM `authz_assignments`
WHERE (`subject_type`, `subject_id`, `role_id`, `tenant_id`) IN (('user', '10001', 900000001, 'platform'),
                                                                 ('user', '10001', 2, 'fangcun'),
                                                                 ('user', '10001', 900000101, '1'),
                                                                 ('user', '110001', 2, 'fangcun'),
                                                                 ('user', '110001', 900000101, '1'),
                                                                 ('user', '110002', 900000102, '1'));

DELETE FROM `authz_policy_versions`
WHERE (`tenant_id`, `policy_version`) IN (('platform', 1),
                                          ('fangcun', 1),
                                          ('1', 1));

DELETE FROM `authz_resources`
WHERE `key` IN ('iam:profile',
                'iam:users',
                'iam:profiles',
                'iam:profile-links',
                'iam:roles',
                'iam:assignments',
                'iam:policies',
                'iam:resources',
                'iam:check',
                'iam:accounts',
                'iam:jwks',
                'iam:wechat_apps',
                'qs:questionnaires',
                'qs:scales',
                'qs:answersheets',
                'qs:assessments',
                'qs:reports',
                'qs:testees',
                'qs:staff',
                'qs:evaluation_plans',
                'qs:evaluation_plan_tasks',
                'qs:system_statistics',
                'qs:statistics_jobs',
                'qs:codes');

DELETE FROM `authz_roles`
WHERE (`tenant_id`, `name`) IN (('platform', 'super_admin'),
                                ('platform', 'platform:admin'),
                                ('platform', 'iam:admin'),
                                ('fangcun', 'super_admin'),
                                ('fangcun', 'tenant_admin'),
                                ('fangcun', 'user'),
                                ('1', 'qs:admin'),
                                ('1', 'qs:content_manager'),
                                ('1', 'qs:evaluator'),
                                ('1', 'qs:staff'),
                                ('1', 'qs:evaluation_plan_manager'));

DELETE FROM `idp_wechat_apps`
WHERE `app_id` = 'wx72ade250b619a649';

DELETE FROM `auth_credentials`
WHERE (`account_id`, `type`) IN ((910100001, 'password'),
                                 (910100002, 'password'),
                                 (910100003, 'password'));

DELETE FROM `auth_accounts`
WHERE (`type`, `app_id`, `external_id`) IN (('opera', 'opera', 'system@fangcunmount.com'),
                                            ('opera', 'opera', 'admin@fangcunmount.com'),
                                            ('opera', 'opera', 'content_manager@fangcunmount.com'));

DELETE FROM `users`
WHERE `id` IN (10001, 110001, 110002);

DELETE FROM `data_dictionary`
WHERE (`dict_type`, `dict_code`) IN (('gender', '0'),
                                     ('gender', '1'),
                                     ('gender', '2'),
                                     ('user_status', '1'),
                                     ('user_status', '2'),
                                     ('user_status', '3'),
                                     ('relation_type', 'father'),
                                     ('relation_type', 'mother'),
                                     ('relation_type', 'grandfather'),
                                     ('relation_type', 'grandmother'),
                                     ('relation_type', 'other'));

DELETE FROM `tenants`
WHERE `id` IN ('platform', 'fangcun');
