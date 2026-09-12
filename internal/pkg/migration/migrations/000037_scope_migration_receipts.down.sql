-- Scope data rollback must preserve its audit receipt. Schema rollback requires
-- a separately verified archive and coordinated restore, never silent deletion.
SELECT iam_scope_receipt_requires_coordinated_restore();
