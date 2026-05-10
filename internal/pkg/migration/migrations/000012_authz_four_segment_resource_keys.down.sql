UPDATE `casbin_rule`
SET `v2` = CASE `v2`
    WHEN '*:*:*:*' THEN '*'
    WHEN 'qs:*:*:*' THEN 'qs:*'
    WHEN 'iam:identity:instance:profile' THEN 'iam:profile'
    WHEN 'iam:identity:collection:users' THEN 'iam:users'
    WHEN 'iam:identity:collection:profiles' THEN 'iam:profiles'
    WHEN 'iam:identity:collection:profile-links' THEN 'iam:profile-links'
    WHEN 'iam:authz:collection:roles' THEN 'iam:roles'
    WHEN 'iam:authz:collection:assignments' THEN 'iam:assignments'
    WHEN 'iam:authz:collection:policies' THEN 'iam:policies'
    WHEN 'iam:authz:collection:resources' THEN 'iam:resources'
    WHEN 'iam:authz:action:check' THEN 'iam:check'
    WHEN 'iam:authn:collection:login_identities' THEN 'iam:login_identities'
    WHEN 'iam:authn:collection:jwks' THEN 'iam:jwks'
    WHEN 'iam:idp:collection:wechat_apps' THEN 'iam:wechat_apps'
    WHEN 'qs:questionnaire:collection:questionnaires' THEN 'qs:questionnaires'
    WHEN 'qs:scale:collection:scales' THEN 'qs:scales'
    WHEN 'qs:answersheet:collection:answersheets' THEN 'qs:answersheets'
    WHEN 'qs:evaluation:collection:assessments' THEN 'qs:assessments'
    WHEN 'qs:evaluation:collection:reports' THEN 'qs:reports'
    WHEN 'qs:actor:collection:testees' THEN 'qs:testees'
    WHEN 'qs:actor:collection:staff' THEN 'qs:staff'
    WHEN 'qs:plan:collection:evaluation_plans' THEN 'qs:evaluation_plans'
    WHEN 'qs:plan_task:collection:evaluation_plan_tasks' THEN 'qs:evaluation_plan_tasks'
    WHEN 'qs:statistics:collection:system_statistics' THEN 'qs:system_statistics'
    WHEN 'qs:statistics:collection:statistics_jobs' THEN 'qs:statistics_jobs'
    WHEN 'qs:code:collection:codes' THEN 'qs:codes'
    ELSE `v2`
END
WHERE `ptype` = 'p'
  AND `v2` IN (
    '*:*:*:*', 'qs:*:*:*', 'iam:identity:instance:profile',
    'iam:identity:collection:users', 'iam:identity:collection:profiles',
    'iam:identity:collection:profile-links', 'iam:authz:collection:roles',
    'iam:authz:collection:assignments', 'iam:authz:collection:policies',
    'iam:authz:collection:resources', 'iam:authz:action:check',
    'iam:authn:collection:login_identities', 'iam:authn:collection:jwks',
    'iam:idp:collection:wechat_apps', 'qs:questionnaire:collection:questionnaires',
    'qs:scale:collection:scales', 'qs:answersheet:collection:answersheets',
    'qs:evaluation:collection:assessments', 'qs:evaluation:collection:reports',
    'qs:actor:collection:testees', 'qs:actor:collection:staff',
    'qs:plan:collection:evaluation_plans', 'qs:plan_task:collection:evaluation_plan_tasks',
    'qs:statistics:collection:system_statistics', 'qs:statistics:collection:statistics_jobs',
    'qs:code:collection:codes'
  );

UPDATE `authz_resources`
SET `key` = CASE `key`
    WHEN 'iam:identity:instance:profile' THEN 'iam:profile'
    WHEN 'iam:identity:collection:users' THEN 'iam:users'
    WHEN 'iam:identity:collection:profiles' THEN 'iam:profiles'
    WHEN 'iam:identity:collection:profile-links' THEN 'iam:profile-links'
    WHEN 'iam:authz:collection:roles' THEN 'iam:roles'
    WHEN 'iam:authz:collection:assignments' THEN 'iam:assignments'
    WHEN 'iam:authz:collection:policies' THEN 'iam:policies'
    WHEN 'iam:authz:collection:resources' THEN 'iam:resources'
    WHEN 'iam:authz:action:check' THEN 'iam:check'
    WHEN 'iam:authn:collection:login_identities' THEN 'iam:login_identities'
    WHEN 'iam:authn:collection:jwks' THEN 'iam:jwks'
    WHEN 'iam:idp:collection:wechat_apps' THEN 'iam:wechat_apps'
    WHEN 'qs:questionnaire:collection:questionnaires' THEN 'qs:questionnaires'
    WHEN 'qs:scale:collection:scales' THEN 'qs:scales'
    WHEN 'qs:answersheet:collection:answersheets' THEN 'qs:answersheets'
    WHEN 'qs:evaluation:collection:assessments' THEN 'qs:assessments'
    WHEN 'qs:evaluation:collection:reports' THEN 'qs:reports'
    WHEN 'qs:actor:collection:testees' THEN 'qs:testees'
    WHEN 'qs:actor:collection:staff' THEN 'qs:staff'
    WHEN 'qs:plan:collection:evaluation_plans' THEN 'qs:evaluation_plans'
    WHEN 'qs:plan_task:collection:evaluation_plan_tasks' THEN 'qs:evaluation_plan_tasks'
    WHEN 'qs:statistics:collection:system_statistics' THEN 'qs:system_statistics'
    WHEN 'qs:statistics:collection:statistics_jobs' THEN 'qs:statistics_jobs'
    WHEN 'qs:code:collection:codes' THEN 'qs:codes'
    ELSE `key`
END
WHERE `key` IN (
    'iam:identity:instance:profile', 'iam:identity:collection:users',
    'iam:identity:collection:profiles', 'iam:identity:collection:profile-links',
    'iam:authz:collection:roles', 'iam:authz:collection:assignments',
    'iam:authz:collection:policies', 'iam:authz:collection:resources',
    'iam:authz:action:check', 'iam:authn:collection:login_identities',
    'iam:authn:collection:jwks', 'iam:idp:collection:wechat_apps',
    'qs:questionnaire:collection:questionnaires', 'qs:scale:collection:scales',
    'qs:answersheet:collection:answersheets', 'qs:evaluation:collection:assessments',
    'qs:evaluation:collection:reports', 'qs:actor:collection:testees',
    'qs:actor:collection:staff', 'qs:plan:collection:evaluation_plans',
    'qs:plan_task:collection:evaluation_plan_tasks',
    'qs:statistics:collection:system_statistics', 'qs:statistics:collection:statistics_jobs',
    'qs:code:collection:codes'
);
