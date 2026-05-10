UPDATE `authz_resources`
SET `key` = CASE `key`
    WHEN 'iam:profile' THEN 'iam:identity:instance:profile'
    WHEN 'iam:users' THEN 'iam:identity:collection:users'
    WHEN 'iam:profiles' THEN 'iam:identity:collection:profiles'
    WHEN 'iam:profile-links' THEN 'iam:identity:collection:profile-links'
    WHEN 'iam:roles' THEN 'iam:authz:collection:roles'
    WHEN 'iam:assignments' THEN 'iam:authz:collection:assignments'
    WHEN 'iam:policies' THEN 'iam:authz:collection:policies'
    WHEN 'iam:resources' THEN 'iam:authz:collection:resources'
    WHEN 'iam:check' THEN 'iam:authz:action:check'
    WHEN 'iam:login_identities' THEN 'iam:authn:collection:login_identities'
    WHEN 'iam:jwks' THEN 'iam:authn:collection:jwks'
    WHEN 'iam:wechat_apps' THEN 'iam:idp:collection:wechat_apps'
    WHEN 'qs:questionnaires' THEN 'qs:questionnaire:collection:questionnaires'
    WHEN 'qs:scales' THEN 'qs:scale:collection:scales'
    WHEN 'qs:answersheets' THEN 'qs:answersheet:collection:answersheets'
    WHEN 'qs:assessments' THEN 'qs:evaluation:collection:assessments'
    WHEN 'qs:reports' THEN 'qs:evaluation:collection:reports'
    WHEN 'qs:testees' THEN 'qs:actor:collection:testees'
    WHEN 'qs:staff' THEN 'qs:actor:collection:staff'
    WHEN 'qs:evaluation_plans' THEN 'qs:plan:collection:evaluation_plans'
    WHEN 'qs:evaluation_plan_tasks' THEN 'qs:plan_task:collection:evaluation_plan_tasks'
    WHEN 'qs:system_statistics' THEN 'qs:statistics:collection:system_statistics'
    WHEN 'qs:statistics_jobs' THEN 'qs:statistics:collection:statistics_jobs'
    WHEN 'qs:codes' THEN 'qs:code:collection:codes'
    ELSE `key`
END
WHERE `key` IN (
    'iam:profile', 'iam:users', 'iam:profiles', 'iam:profile-links',
    'iam:roles', 'iam:assignments', 'iam:policies', 'iam:resources',
    'iam:check', 'iam:login_identities', 'iam:jwks', 'iam:wechat_apps',
    'qs:questionnaires', 'qs:scales', 'qs:answersheets', 'qs:assessments',
    'qs:reports', 'qs:testees', 'qs:staff', 'qs:evaluation_plans',
    'qs:evaluation_plan_tasks', 'qs:system_statistics', 'qs:statistics_jobs',
    'qs:codes'
);

UPDATE `casbin_rule`
SET `v2` = CASE `v2`
    WHEN '*' THEN '*:*:*:*'
    WHEN 'qs:*' THEN 'qs:*:*:*'
    WHEN 'iam:profile' THEN 'iam:identity:instance:profile'
    WHEN 'iam:users' THEN 'iam:identity:collection:users'
    WHEN 'iam:profiles' THEN 'iam:identity:collection:profiles'
    WHEN 'iam:profile-links' THEN 'iam:identity:collection:profile-links'
    WHEN 'iam:roles' THEN 'iam:authz:collection:roles'
    WHEN 'iam:assignments' THEN 'iam:authz:collection:assignments'
    WHEN 'iam:policies' THEN 'iam:authz:collection:policies'
    WHEN 'iam:resources' THEN 'iam:authz:collection:resources'
    WHEN 'iam:check' THEN 'iam:authz:action:check'
    WHEN 'iam:login_identities' THEN 'iam:authn:collection:login_identities'
    WHEN 'iam:jwks' THEN 'iam:authn:collection:jwks'
    WHEN 'iam:wechat_apps' THEN 'iam:idp:collection:wechat_apps'
    WHEN 'qs:questionnaires' THEN 'qs:questionnaire:collection:questionnaires'
    WHEN 'qs:scales' THEN 'qs:scale:collection:scales'
    WHEN 'qs:answersheets' THEN 'qs:answersheet:collection:answersheets'
    WHEN 'qs:assessments' THEN 'qs:evaluation:collection:assessments'
    WHEN 'qs:reports' THEN 'qs:evaluation:collection:reports'
    WHEN 'qs:testees' THEN 'qs:actor:collection:testees'
    WHEN 'qs:staff' THEN 'qs:actor:collection:staff'
    WHEN 'qs:evaluation_plans' THEN 'qs:plan:collection:evaluation_plans'
    WHEN 'qs:evaluation_plan_tasks' THEN 'qs:plan_task:collection:evaluation_plan_tasks'
    WHEN 'qs:system_statistics' THEN 'qs:statistics:collection:system_statistics'
    WHEN 'qs:statistics_jobs' THEN 'qs:statistics:collection:statistics_jobs'
    WHEN 'qs:codes' THEN 'qs:code:collection:codes'
    ELSE `v2`
END
WHERE `ptype` = 'p'
  AND `v2` IN (
    '*', 'qs:*', 'iam:profile', 'iam:users', 'iam:profiles', 'iam:profile-links',
    'iam:roles', 'iam:assignments', 'iam:policies', 'iam:resources',
    'iam:check', 'iam:login_identities', 'iam:jwks', 'iam:wechat_apps',
    'qs:questionnaires', 'qs:scales', 'qs:answersheets', 'qs:assessments',
    'qs:reports', 'qs:testees', 'qs:staff', 'qs:evaluation_plans',
    'qs:evaluation_plan_tasks', 'qs:system_statistics', 'qs:statistics_jobs',
    'qs:codes'
  );
