DELETE FROM `casbin_rule`
WHERE (`ptype`, `v0`, `v1`, `v2`, `v3`) IN (('p', 'role:super_admin', 'platform', '*:*:*:*', '.*'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:identity:collection:users',
                                             'read|search|create|update|deactivate|block|link_external_identity'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:identity:collection:profiles',
                                             'read|list|search|create|update'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:identity:collection:profile-links',
                                             'read|list|grant|update_relation|revoke|bulk_revoke|import'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:authz:collection:roles',
                                             'create|read|update|delete|list'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:authz:collection:assignments',
                                             'grant|revoke|delete|read'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:authz:collection:policies', 'read|write|delete'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:authz:collection:resources',
                                             'create|read|update|delete|list|validate_action'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:authz:action:check', 'check'),
                                            ('p', 'role:tenant_admin', 'fangcun', 'iam:authn:collection:login_identities',
                                             'read|update|enable|disable'),
                                            ('p', 'role:user', 'fangcun', 'iam:identity:instance:profile', 'read'),
                                            ('p', 'role:user', 'fangcun', 'iam:identity:instance:profile', 'update'),
                                            ('p', 'role:qs:admin', '1', 'qs:*:*:*', '.*'),
                                            ('p', 'role:qs:content_manager', '1', 'qs:questionnaire:collection:questionnaires',
                                             'create|read|list|update|delete|publish|unpublish|archive|statistics'),
                                            ('p', 'role:qs:content_manager', '1', 'qs:scale:collection:scales',
                                             'create|read|list|update|delete|publish|unpublish|archive'),
                                            ('p', 'role:qs:evaluator', '1', 'qs:answersheet:collection:answersheets',
                                             'read|list|statistics'),
                                            ('p', 'role:qs:evaluator', '1', 'qs:evaluation:collection:assessments',
                                             'read|list|retry|batch_evaluate|statistics'),
                                            ('p', 'role:qs:evaluator', '1', 'qs:evaluation:collection:reports', 'read|list'),
                                            ('p', 'role:qs:evaluator', '1', 'qs:actor:collection:testees',
                                             'read|list|analyze|statistics'),
                                            ('p', 'role:qs:staff', '1', 'qs:actor:collection:testees', 'read|list'),
                                            ('p', 'role:qs:evaluation_plan_manager', '1', 'qs:plan:collection:evaluation_plans',
                                             'create|read|list|update|pause|resume|cancel|enroll|terminate|statistics'),
                                            ('p', 'role:qs:evaluation_plan_manager', '1',
                                             'qs:plan_task:collection:evaluation_plan_tasks',
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
WHERE `key` IN ('iam:identity:instance:profile',
                'iam:identity:collection:users',
                'iam:identity:collection:profiles',
                'iam:identity:collection:profile-links',
                'iam:authz:collection:roles',
                'iam:authz:collection:assignments',
                'iam:authz:collection:policies',
                'iam:authz:collection:resources',
                'iam:authz:action:check',
                'iam:authn:collection:login_identities',
                'iam:authn:collection:jwks',
                'iam:idp:collection:wechat_apps',
                'qs:questionnaire:collection:questionnaires',
                'qs:scale:collection:scales',
                'qs:answersheet:collection:answersheets',
                'qs:evaluation:collection:assessments',
                'qs:evaluation:collection:reports',
                'qs:actor:collection:testees',
                'qs:actor:collection:staff',
                'qs:plan:collection:evaluation_plans',
                'qs:plan_task:collection:evaluation_plan_tasks',
                'qs:statistics:collection:system_statistics',
                'qs:statistics:collection:statistics_jobs',
                'qs:code:collection:codes');

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
WHERE (`login_identity_id`, `type`) IN ((910100001, 'password'),
                                        (910100002, 'password'),
                                        (910100003, 'password'));

DELETE FROM `auth_login_identities`
WHERE (`provider`, `realm`, `identifier`) IN (('username', '1', 'system@fangcunmount.com'),
                                              ('username', '1', 'admin@fangcunmount.com'),
                                              ('username', '1', 'content_manager@fangcunmount.com'));

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
