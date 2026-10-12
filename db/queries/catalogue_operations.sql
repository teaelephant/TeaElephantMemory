-- name: GetCatalogueOperationReceipt :one
SELECT operation_key,actor,operation,request_hash,tea_id,resource_id,result_revision,completed_at
FROM catalog_operation_receipts WHERE operation_key=$1;

-- name: InsertCatalogueOperationReceipt :exec
INSERT INTO catalog_operation_receipts(operation_key,actor,operation,request_hash,tea_id,resource_id,result_revision)
VALUES($1,$2,$3,$4,$5,$6,$7);
