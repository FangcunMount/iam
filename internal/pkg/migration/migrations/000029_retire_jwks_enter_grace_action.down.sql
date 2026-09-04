UPDATE authz_resources
SET actions = JSON_ARRAY_APPEND(actions, '$', 'enter_grace')
WHERE resource_key = 'iam:authn:collection:jwks'
  AND JSON_SEARCH(actions, 'one', 'enter_grace') IS NULL;
