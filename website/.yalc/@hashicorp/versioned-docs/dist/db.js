"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.isVersionUploaded = exports.retrieveDocument = exports.writeDocument = void 0;
const client_sts_1 = require("@aws-sdk/client-sts");
const client_dynamodb_1 = require("@aws-sdk/client-dynamodb");
const util_dynamodb_1 = require("@aws-sdk/util-dynamodb");
const transforms_1 = require("./transforms");
// TODO: can we somehow use TF to set an env var for these for our vercel project?
const TABLE_NAME = 'test-versioned-docs-poc-stirring-piglet';
const ROLE_ARN = 'arn:aws:iam::781101930495:role/webplatform-docs';
const AWS_REGION = 'us-west-1';
let client;
async function dynamoClient() {
    if (client) {
        return client;
    }
    const stsClient = new client_sts_1.STSClient({
        region: AWS_REGION,
        credentials: {
            accessKeyId: process.env.HC_AWS_ACCESS_KEY_ID ?? '',
            secretAccessKey: process.env.HC_AWS_SECRET_ACCESS_KEY ?? '',
        },
    });
    const roleResult = await stsClient.send(new client_sts_1.AssumeRoleCommand({
        RoleArn: ROLE_ARN,
        RoleSessionName: 'mktg-versioned-docs-poc',
    }));
    client = new client_dynamodb_1.DynamoDBClient({
        region: AWS_REGION,
        credentials: {
            accessKeyId: roleResult?.Credentials?.AccessKeyId ?? '',
            secretAccessKey: roleResult?.Credentials?.SecretAccessKey ?? '',
            sessionToken: roleResult?.Credentials?.SessionToken,
            expiration: roleResult?.Credentials?.Expiration,
        },
    });
    return client;
}
async function writeDocument(document) {
    const client = await dynamoClient();
    const command = new client_dynamodb_1.PutItemCommand({
        TableName: TABLE_NAME,
        Item: util_dynamodb_1.marshall({ ...document, created_at: String(new Date()) }),
    });
    try {
        await client.send(command);
    }
    catch (err) {
        console.error('Error writing document to dynamo:', err);
    }
}
exports.writeDocument = writeDocument;
// TODO: error handling
async function retrieveDocument(product, fullPath) {
    const client = await dynamoClient();
    const command = new client_dynamodb_1.QueryCommand({
        TableName: TABLE_NAME,
        KeyConditionExpression: 'product = :p AND fullPath = :v',
        ExpressionAttributeValues: {
            ':p': { S: product },
            ':v': { S: fullPath },
        },
    });
    const result = await client.send(command);
    if (result.Items) {
        const document = result.Items[0];
        return util_dynamodb_1.unmarshall(document);
    }
}
exports.retrieveDocument = retrieveDocument;
// TODO: error handling
async function isVersionUploaded(product, sha) {
    const client = await dynamoClient();
    const command = new client_dynamodb_1.QueryCommand({
        TableName: TABLE_NAME,
        IndexName: 'productAndShaIndex',
        KeyConditionExpression: 'product = :p AND sha = :s',
        ExpressionAttributeValues: {
            ':p': { S: product },
            ':s': { S: sha },
        },
        Limit: 1,
    });
    const result = await client.send(command);
    if ((result?.Count ?? 0) === 0)
        return false;
    const document = util_dynamodb_1.unmarshall(result.Items[0]);
    if ([...document.mdxTransforms].sort().join(',') ===
        [...transforms_1.BASE_TRANSFORMS].sort().join(','))
        return true;
}
exports.isVersionUploaded = isVersionUploaded;
