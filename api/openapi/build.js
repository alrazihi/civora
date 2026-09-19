const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

const ROOT = path.join(__dirname);
const OPENAPI_FILE = path.join(ROOT, 'openapi.yaml');
const SCHEMAS_DIR = path.join(ROOT, 'schemas');
const PATHS_DIR = path.join(ROOT, 'paths');
const RESPONSES_DIR = path.join(ROOT, 'responses');

function ensureDir(dir) {
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }
}

function sanitizeName(name) {
    return name.replace(/[{}]/g, '').replace(/\//g, '_');
}

function pathFileNameToPath(fileName) {
    const base = fileName.replace(/\.yaml$/, '');
    const replacements = [
        ['org-id', '{orgId}'],
        ['case-id', '{caseId}'],
        ['evidence-id', '{evidenceId}'],
        ['document-id', '{documentId}'],
        ['observation-id', '{observationId}'],
        ['summary-id', '{summaryId}'],
        ['rule-set-id', '{ruleSetId}'],
        ['rule-set-key-key', 'rule-sets/key/{key}'],
        ['workflow-id', '{workflowId}'],
        ['assignment-id', '{assignmentId}'],
        ['state-state-key', 'form-assignments/state/{stateKey}'],
        ['submission-id', '{submissionId}'],
        ['form-key-submission', 'form/{formKey}/submission'],
        ['review-id', '{reviewId}'],
        ['assistance-id', '{assistanceId}'],
        ['follow-up-id', '{followUpId}'],
        ['form-id', '{formId}'],
        ['field-id', '{fieldId}'],
        ['person-id', '{personId}'],
        ['external-ref', 'external/{externalRef}'],
        ['template-id', '{templateId}'],
        ['user-id', '{userId}'],
        ['assessment-id', '{assessmentId}'],
        ['service-request-id', 'by-service-request/{serviceRequestId}'],
        ['eligibility-id', '{eligibilityId}'],
        ['decision-id', '{decisionId}'],
        ['transition-key', '{transitionKey}'],
        ['version-id', '{versionId}'],
        ['active-version', 'active-version'],
        ['instantiate', 'instantiate'],
        ['publish', 'publish'],
        ['archive', 'archive'],
        ['activate', 'activate'],
        ['assign', 'assign'],
        ['claim', 'claim'],
        ['complete', 'complete'],
        ['start', 'start'],
        ['escalate', 'escalate'],
        ['request-information', 'request-information'],
        ['review', 'review'],
        ['verify', 'verify'],
        ['upload', 'upload'],
        ['reject', 'reject'],
        ['download', 'download'],
        ['history', 'history'],
        ['selectable', 'selectable'],
        ['forms', 'forms'],
        ['fields', 'fields'],
        ['versions', 'versions'],
        ['evaluations', 'evaluations'],
        ['templates', 'templates'],
        ['assistances', 'assistances'],
        ['follow-ups', 'follow-ups'],
        ['review-queue', 'review-queue'],
        ['decisions', 'decisions'],
        ['eligibilities', 'eligibilities'],
        ['documents', 'documents'],
        ['assessments', 'assessments'],
        ['people', 'people'],
        ['users', 'users'],
        ['workflows', 'workflows'],
        ['rules', 'rules'],
        ['cases', 'cases'],
        ['auth', 'auth'],
        ['audit', 'audit'],
        ['ready', 'ready'],
        ['health', 'health'],
        ['organizations', 'organizations'],
    ];

    let result = base;
    for (const [from, to] of replacements) {
        result = result.replace(new RegExp(from, 'g'), to);
    }
    return '/' + result.replace(/_/g, '-').replace(/--+/g, '-');
}

function readYamlFiles(dir) {
    const result = {};
    if (!fs.existsSync(dir)) return result;

    const files = fs.readdirSync(dir).filter(f => f.endsWith('.yaml'));
    for (const file of files) {
        const filePath = path.join(dir, file);
        const content = fs.readFileSync(filePath, 'utf8');
        const doc = yaml.load(content);
        const name = file.replace(/\.yaml$/, '');
        result[name] = doc;
    }
    return result;
}

function buildOpenAPI() {
    console.log('Building', OPENAPI_FILE, '...');

    const schemas = readYamlFiles(SCHEMAS_DIR);
    const responses = readYamlFiles(RESPONSES_DIR);
    const pathFiles = readYamlFiles(PATHS_DIR);

    const paths = {};
    for (const [fileName, pathItem] of Object.entries(pathFiles)) {
        const pathStr = pathFileNameToPath(fileName);
        paths[pathStr] = pathItem;
    }

    const openapi = {
        openapi: '3.0.3',
        info: {
            title: 'CIVORA API',
            description: 'CIVORA API - open-source infrastructure for configurable, auditable public-service workflows, case management, forms, rules, and human decisions.',
            version: '0.9.0',
            contact: {
                name: 'CIVORA',
                url: 'https://github.com/alrazihi/civora'
            },
            license: {
                name: 'Apache-2.0',
                url: 'https://www.apache.org/licenses/LICENSE-2.0'
            }
        },
        servers: [
            {
                url: 'http://localhost:8080',
                description: 'Local development server. Override with CIVORA_API_URL in production.'
            }
        ],
        security: [],
        tags: [
            { name: 'Organizations', description: 'Organization (tenant) management' },
            { name: 'Authentication', description: 'User registration and authentication' },
            { name: 'Users', description: 'User management within an organization' },
            { name: 'People', description: 'Person (client/individual) records' },
            { name: 'Cases', description: 'Case lifecycle management with state transitions' },
            { name: 'Eligibilities', description: 'Eligibility assessment for service requests' },
            { name: 'Evidence', description: 'Evidence items submitted for service requests' },
            { name: 'Assessments', description: 'Needs assessment and recommendations' },
            { name: 'Decisions', description: 'Human decisions on service requests' },
            { name: 'Assistance', description: 'Assistance actions and services provided' },
            { name: 'FollowUps', description: 'Follow-up scheduling and completion' },
            { name: 'Audit', description: 'Immutable audit event log' },
            { name: 'Workflows', description: 'Configurable workflow definitions, instances, and transitions' },
            { name: 'Forms', description: 'Configurable forms for data collection' },
            { name: 'Rules', description: 'Deterministic eligibility rules engine configuration and evaluation' },
            { name: 'ReviewQueue', description: 'Human reviewer work queue and decision tracking' },
            { name: 'AI', description: 'AI observation generation, review, and provenance' },
            { name: 'Operations', description: 'Operational metrics and impact intelligence' },
            { name: 'DocumentIntelligence', description: 'AI document analysis capabilities' },
            { name: 'CaseContext', description: 'Case context building and fact management' },
            { name: 'System', description: 'Health and readiness checks' }
        ],
        components: {
            securitySchemes: {
                BearerAuth: {
                    type: 'http',
                    scheme: 'bearer',
                    bearerFormat: 'JWT',
                    description: 'JWT bearer token. The token must include `organization_id` claim. The API validates that the token\'s organization matches the `{orgId}` path parameter on all tenant-scoped endpoints.'
                }
            },
            headers: {
                'X-Request-ID': {
                    schema: {
                        type: 'string',
                        format: 'uuid',
                        pattern: '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'
                    },
                    description: 'Correlation ID for the request. Generated if not provided by the client.'
                }
            },
            schemas: schemas,
            responses: responses
        },
        paths: paths
    };

    const output = yaml.dump(openapi, { indent: 2, lineWidth: 120, noRefs: true });
    fs.writeFileSync(OPENAPI_FILE, output);
    console.log('Build complete:', OPENAPI_FILE);
}

function usage() {
    console.log('Usage: node build-openapi.js [split|build]');
    console.log('  split  - Extract modular files from bundled openapi.yaml');
    console.log('  build  - Assemble modular files into bundled openapi.yaml');
    process.exit(1);
}

const command = process.argv[2];
if (command === 'split') {
    splitOpenAPI();
} else if (command === 'build') {
    buildOpenAPI();
} else {
    usage();
}
