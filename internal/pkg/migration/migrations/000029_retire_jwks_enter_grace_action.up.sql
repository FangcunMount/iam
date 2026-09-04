UPDATE `authz_resources`
SET `actions` = JSON_REMOVE(
        `actions`,
        JSON_UNQUOTE(JSON_SEARCH(`actions`, 'one', 'enter_grace'))
    )
WHERE `key` = 'iam:authn:collection:jwks'
  AND JSON_SEARCH(`actions`, 'one', 'enter_grace') IS NOT NULL;
