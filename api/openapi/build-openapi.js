const fs = require('fs');
const path = require('path');

const OPENAPI_FILE = path.join(__dirname, 'openapi.yaml');
const MODULAR_DIR = path.join(__dirname, 'modular');
const BUILD_OUTPUT = path.join(__dirname, 'openapi.yaml');

function parseYaml(content) {
    const lines = content.split('\n');
    const result = {
        headers: {},
        components: { securitySchemes: {}, headers: {}, schemas: {}, responses: {} },
        paths: {}
    };
    
    let currentSection = null;
    let currentPath = null;
    let indentLevel = 0;
    let pathIndentLevel = 0;
    let schemaName = null;
    
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const trimmed = line.trim();
        
        if (trimmed.startsWith('components:') || trimmed.startsWith('schemas:') || 
            trimmed.startsWith('responses:') || trimmed.startsWith('paths:') ||
            trimmed.startsWith('securitySchemes:') || trimmed.startsWith('headers:')) {
            currentSection = trimmed.replace(':', '');
            continue;
        }
        
        if (trimmed.startsWith('/')) {
            pathIndentLevel = getIndent(line);
            currentPath = trimmed;
            result.paths[currentPath] = [];
            continue;
        }
        
        if (currentSection === 'schemas' || currentSection === 'responses' || currentSection === 'securitySchemes') {
            const match = trimmed.match(/^(\w+):/);
            if (match && !trimmed.includes('$ref') && !trimmed.includes('type:') && !trimmed.includes('properties:')) {
                schemaName = match[1];
                if (currentSection === 'schemas') {
                    result.components.schemas[schemaName] = { startLine: i, name: schemaName };
                } else if (currentSection === 'responses') {
                    result.components.responses[schemaName] = { startLine: i, name: schemaName };
                } else if (currentSection === 'securitySchemes') {
                    result.components.securitySchemes[schemaName] = { startLine: i, name: schemaName };
                }
            }
        }
    }
    
    return result;
}

function getIndent(line) {
    let indent = 0;
    for (const char of line) {
        if (char === ' ') indent++;
        else break;
    }
    return indent;
}

function extractSchema(lines, startIdx, nextIdx) {
    const schemaLines = [];
    let baseIndent = 100;
    
    for (let i = startIdx; i < nextIdx; i++) {
        const line = lines[i];
        const indent = getIndent(line);
        if (indent < baseIndent && trimmedLine(line).trim() !== '') {
            baseIndent = indent;
        }
    }
    
    for (let i = startIdx; i < nextIdx; i++) {
        const line = lines[i];
        const indent = getIndent(line);
        if (trimmedLine(line).trim() === '' || indent >= baseIndent) {
            schemaLines.push(line);
        }
    }
    
    return schemaLines.join('\n');
}

function trimmedLine(line) {
    return line.trim();
}

function modularizeOpenAPI() {
    const content = fs.readFileSync(OPENAPI_FILE, 'utf8');
    const lines = content.split('\n');
    
    if (!fs.existsSync(MODULAR_DIR)) {
        fs.mkdirSync(MODULAR_DIR, { recursive: true });
    }
    
    const schemasDir = path.join(MODULAR_DIR, 'schemas');
    const pathsDir = path.join(MODULAR_DIR, 'paths');
    const componentsDir = path.join(MODULAR_DIR, 'components');
    
    fs.mkdirSync(schemasDir, { recursive: true });
    fs.mkdirSync(pathsDir, { recursive: true });
    fs.mkdirSync(componentsDir, { recursive: true });
    
    let currentPath = null;
    let pathStartLine = -1;
    const paths = [];
    
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const trimmed = line.trim();
        
        if (trimmed === 'paths:') continue;
        if (trimmed === 'components:') continue;
        if (trimmed === 'securitySchemes:') continue;
        if (trimmed === 'headers:') continue;
        if (trimmed === 'schemas:') continue;
        if (trimmed === 'responses:') continue;
        
        if (trimmed.startsWith('openapi:')) {
            writeMainFile(lines, schemasDir, pathsDir, content);
            break;
        }
        
        if (trimmed.startsWith('http://') || trimmed.startsWith('https://') || trimmed.startsWith('{orgId}')) {
            if (trimmed.includes(':') && !line.includes('  -')) {
                currentPath = trimmed;
                pathStartLine = i;
                paths.push(currentPath);
                extractPath(lines, i, currentPath);
            }
        }
    }
    
    console.log('Modularization complete!');
    console.log(`Created ${paths.length} path files`);
}

function writeMainFile(lines, schemasDir, pathsDir, content) {
    const mainContent = `# OpenAPI 3.0.3 Specification - CIVORA API
# This file uses \$ref to external files for maintainability

openapi: 3.0.3
info:
  title: CIVORA API
  description: >-
      CIVORA API - open-source infrastructure for configurable, auditable
      public-service workflows, case management, forms, rules, and human
      decisions. Version 0.7.0.
  version: 0.8.0
  contact:
    name: CIVORA
    url: https://github.com/alrazihi/civora
  license:
    name: Apache-2.0
    url: https://www.apache.org/licenses/LICENSE-2.0

servers:
- url: http://localhost:8080
  description: Local development server. Override with CIVORA_API_URL in production.

security: []

tags:
- name: Organizations
  description: Organization (tenant) management
- name: Authentication
  description: User registration and authentication
- name: Users
  description: User management within an organization
- name: People
  description: Person (client/individual) records
- name: Cases
  description: Case lifecycle management with state transitions
- name: Eligibilities
  description: Eligibility assessment for service requests
- name: Evidence
  description: Evidence items submitted for service requests
- name: Assessments
  description: Needs assessment and recommendations
- name: Decisions
  description: Human decisions on service requests
- name: Assistance
  description: Assistance actions and services provided
- name: FollowUps
  description: Follow-up scheduling and completion
- name: Audit
  description: Immutable audit event log
- name: Workflows
  description: Configurable workflow definitions, instances, and transitions
- name: Forms
  description: Configurable forms for data collection
- name: Rules
  description: Deterministic eligibility rules engine configuration and evaluation
- name: ReviewQueue
  description: Human reviewer work queue and decision tracking
- name: AI
  description: AI observation generation, review, and provenance
- name: Operations
  description: Operational metrics and impact intelligence
- name: DocumentIntelligence
  description: AI document analysis capabilities
- name: CaseContext
  description: Case context building and fact management
- name: System
  description: Health and readiness checks

components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: 'JWT bearer token with organization_id claim'
  
  headers:
    X-Request-ID:
      schema:
        type: string
        format: uuid
  
  responses:
    BadRequest:
      description: Bad request
      content:
        application/json:
          schema:
            $ref: './components/responses/Error.yaml'
    Unauthorized:
      description: Authentication required
      content:
        application/json:
          schema:
            $ref: './components/responses/Error.yaml'
    Forbidden:
      description: Access forbidden
      content:
        application/json:
          schema:
            $ref: './components/responses/Error.yaml'
    NotFound:
      description: Resource not found
      content:
        application/json:
          schema:
            $ref: './components/responses/Error.yaml'
    Conflict:
      description: Resource conflict
      content:
        application/json:
          schema:
            $ref: './components/responses/Error.yaml'
    RequestTooLarge:
      description: Request body too large (exceeds 1MB limit)
      content:
        application/json:
          schema:
            $ref: './components/responses/Error.yaml'

  schemas:
    Error:
      $ref: './components/schemas/Error.yaml'

paths:
  /health:
    $ref: './paths/health.yaml'
`;
    
    fs.writeFileSync(BUILD_OUTPUT, mainContent);
    console.log('Main openapi.yaml created');
}

function extractPath(lines, startIdx, path) {
    const pathDir = path.join(__dirname, 'modular', 'paths');
    const fileName = sanitizePathName(path);
    
    const pathContent = [];
    let foundPath = false;
    let indentLevel = 0;
    
    for (let i = startIdx; i < lines.length; i++) {
        const line = lines[i];
        const trimmed = line.trim();
        
        if (!foundPath && (trimmed.startsWith(path) || trimmed.startsWith('/'))) {
            foundPath = true;
            indentLevel = getIndent(line);
        }
        
        if (foundPath) {
            if (trimmed === '' || getIndent(line) >= indentLevel || trimmed.startsWith('/')) {
                pathContent.push(line);
            } else {
                break;
            }
        }
    }
    
    const content = pathContent.join('\n');
    fs.writeFileSync(path.join(pathDir, fileName + '.yaml'), content);
}

function sanitizePathName(pathStr) {
    return pathStr
        .replace(/[{}/]/g, '')
        .replace(/-/g, '_')
        .replace(/{orgId}/g, 'org-id')
        .replace(/{/g, '')
        .replace(/}/g, '');
}

modularizeOpenAPI();