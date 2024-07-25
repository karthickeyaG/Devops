package dynamodbscanner
 
import (
"context"
"log"
"strings"
 
"github.com/aws/aws-sdk-go-v2/aws"
"github.com/aws/aws-sdk-go-v2/service/dynamodb"
dynamodbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
"github.com/aws/aws-sdk-go-v2/service/ec2"
ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)
 
// DynamoDBTable represents DynamoDB table details.
type DynamoDBTable struct {
TableName           string                `json:"tableName"`
PartitionKey        string                `json:"partitionKey"`
CapacityMode        string                `json:"capacityMode"`
StorageType         string                `json:"storageType"`
DataFormat          string                `json:"dataFormat"`
TableStatus         string                `json:"tableStatus"`
TableID             string                `json:"tableID"`
Region              string                `json:"region"`
PointInTimeRecovery bool                  `json:"pointInTimeRecovery"`
DeletionProtection  bool                  `json:"deletionProtection"`
DataRetention       string                `json:"dataRetention"`
AccessControl       string                `json:"accessControl"`
NetworkAccess       string                `json:"networkAccess"`
EncryptionAtRest    string                `json:"encryptionAtRest"`
EncryptionInTransit bool                  `json:"encryptionInTransit"`
BackupPolicy        BackupPolicy          `json:"backupPolicy"`
DataClassification  string                `json:"dataClassification"`
DataOwner           string                `json:"dataOwner"`
}
 
// BackupPolicy represents the backup policy details for the DynamoDB table.
type BackupPolicy struct {
PolicyType string                 `json:"policyType"`
Backups    []dynamodbTypes.BackupDescription `json:"backups"`
}
 
// ScanDynamoDBTables scans all DynamoDB tables in the configured AWS account and region.
func ScanDynamoDBTables(cfg aws.Config, logger *log.Logger) ([]DynamoDBTable, error) {
dynamoSvc := dynamodb.NewFromConfig(cfg)
ec2Svc := ec2.NewFromConfig(cfg)
var tables []DynamoDBTable
 
// List Tables
listTablesOutput, err := dynamoSvc.ListTables(context.TODO(), &dynamodb.ListTablesInput{})
if err != nil {
return nil, err
}
 
logger.Println("DynamoDB Tables:")
for _, tableName := range listTablesOutput.TableNames {
logger.Println("Table Name:", tableName)
 
// Describe Table
describeTableOutput, err := dynamoSvc.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{
TableName: &tableName,
})
if err != nil {
logger.Printf("failed to describe table %s: %v\n", tableName, err)
continue // Continue to the next table on error
}
 
table := describeTableOutput.Table
capacityMode := "UNKNOWN"
if table.BillingModeSummary != nil {
capacityMode = string(table.BillingModeSummary.BillingMode)
}
 
// Get Storage Type and Data Format
storageType, dataFormat := getTableStorageAndFormat(dynamoSvc, tableName)
 
// Get Point-in-Time Recovery status
pointInTimeRecovery := false
pitrOutput, err := dynamoSvc.DescribeContinuousBackups(context.TODO(), &dynamodb.DescribeContinuousBackupsInput{
TableName: &tableName,
})
if err == nil && pitrOutput.ContinuousBackupsDescription.PointInTimeRecoveryDescription != nil {
pointInTimeRecovery = pitrOutput.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus == dynamodbTypes.PointInTimeRecoveryStatusEnabled
}
 
// Get Deletion Protection status
deletionProtection := table.DeletionProtectionEnabled != nil && *table.DeletionProtectionEnabled
 
partitionKey := getAttributeName(table.KeySchema, dynamodbTypes.KeyTypeHash)
 
// Determine network access type (public/private)
networkAccess := "public"
if isPrivateAccess(ec2Svc, cfg.Region) {
networkAccess = "private"
}
 
// Get Encryption at Rest
encryptionAtRest := getEncryptionAtRest(table.SSEDescription)
 
// Encryption in transit is always true for DynamoDB as AWS handles it
encryptionInTransit := true
 
// Backup policy based on presence of continuous backups
backupPolicy := BackupPolicy{
PolicyType: "None",
Backups:    []dynamodbTypes.BackupDescription{},
}
if pointInTimeRecovery {
backupPolicy.PolicyType = "Continuous"
} else {
// List On-Demand Backups
listBackupsOutput, err := dynamoSvc.ListBackups(context.TODO(), &dynamodb.ListBackupsInput{
TableName: &tableName,
})
if err == nil {
for _, backupDetails := range listBackupsOutput.BackupSummaries {
describeBackupOutput, err := dynamoSvc.DescribeBackup(context.TODO(), &dynamodb.DescribeBackupInput{
BackupArn: backupDetails.BackupArn,
})
if err == nil {
backupPolicy.Backups = append(backupPolicy.Backups, describeBackupOutput.BackupDescription)
}
}
if len(backupPolicy.Backups) > 0 {
backupPolicy.PolicyType = "On-Demand"
}
}
}
 
// Data classification placeholder, this needs to be managed via metadata or application-level logic
dataClassification := "N/A"
 
dbTable := DynamoDBTable{
TableName:           tableName,
PartitionKey:        partitionKey,
CapacityMode:        capacityMode,
StorageType:         storageType,
DataFormat:          dataFormat,
TableStatus:         string(table.TableStatus),
TableID:             *table.TableId,
Region:              cfg.Region,
PointInTimeRecovery: pointInTimeRecovery,
DeletionProtection:  deletionProtection,
DataRetention:       "N/A", // DynamoDB does not have specific data retention policies
AccessControl:       "IAM", // Access control is managed via IAM
NetworkAccess:       networkAccess,
EncryptionAtRest:    encryptionAtRest,
EncryptionInTransit: encryptionInTransit,
BackupPolicy:        backupPolicy,
DataClassification:  dataClassification,
DataOwner:           "NA",
}
 
tables = append(tables, dbTable)
 
logger.Printf("Partition Key: %s\n", dbTable.PartitionKey)
logger.Printf("Capacity Mode: %s\n", dbTable.CapacityMode)
logger.Printf("Storage Type: %s\n", dbTable.StorageType)
logger.Printf("Data Format: %s\n", dbTable.DataFormat)
logger.Printf("Table Status: %s\n", dbTable.TableStatus)
logger.Printf("Table ID: %s\n", dbTable.TableID)
logger.Printf("Region: %s\n", dbTable.Region)
logger.Printf("Point In Time Recovery: %t\n", dbTable.PointInTimeRecovery)
logger.Printf("Deletion Protection: %t\n", dbTable.DeletionProtection)
logger.Printf("Access Control: %s\n", dbTable.AccessControl)
logger.Printf("Network Access: %s\n", dbTable.NetworkAccess)
logger.Printf("Encryption At Rest: %s\n", dbTable.EncryptionAtRest)
logger.Printf("Encryption In Transit: %t\n", dbTable.EncryptionInTransit)
logger.Printf("Backup Policy: %s\n", dbTable.BackupPolicy.PolicyType)
logger.Printf("Data Classification: %s\n", dbTable.DataClassification)
logger.Printf("Data Owner: %s\n", dbTable.DataOwner)
logger.Println("-------")
}
 
return tables, nil
}
 
// getAttributeName is a helper function to get the attribute name by key type from KeySchema.
func getAttributeName(keySchema []dynamodbTypes.KeySchemaElement, keyType dynamodbTypes.KeyType) string {
for _, element := range keySchema {
if element.KeyType == keyType {
return *element.AttributeName
}
}
return ""
}
 
// isPrivateAccess checks if there are VPC endpoints configured for DynamoDB in the given region.
func isPrivateAccess(ec2Svc *ec2.Client, region string) bool {
vpcEndpointOutput, err := ec2Svc.DescribeVpcEndpoints(context.TODO(), &ec2.DescribeVpcEndpointsInput{
Filters: []ec2Types.Filter{
{
Name:   aws.String("service-name"),
Values: []string{"com.amazonaws." + region + ".dynamodb"},
},
},
})
if err != nil {
return false
}
 
return len(vpcEndpointOutput.VpcEndpoints) > 0
}
 
// getTableStorageAndFormat fetches the storage type and data format for the DynamoDB table.
func getTableStorageAndFormat(svc *dynamodb.Client, tableName string) (storageType string, dataFormat string) {
// For DynamoDB, storage type is NoSQL and data format is schema-less
return "NoSQL", "Schema-less"
}
 
// getEncryptionAtRest determines the type of encryption at rest for DynamoDB table.
func getEncryptionAtRest(sseDescription *dynamodbTypes.SSEDescription) string {
if sseDescription == nil {
return "Owned by Amazon DynamoDB"
}
 
kmsArn := ""
if sseDescription.KMSMasterKeyArn != nil {
kmsArn = *sseDescription.KMSMasterKeyArn
}
 
// Check if KMSArn starts with "arn:aws:kms"
if strings.HasPrefix(kmsArn, "arn:aws:kms") {
return "AWS managed key"
}
 
// Check if KMSArn starts with "alias/"
if strings.HasPrefix(kmsArn, "alias/") {
return "Customer managed key"
}
 
return "Owned by Amazon DynamoDB"
}