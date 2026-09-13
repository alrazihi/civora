#!/usr/bin/env python3
# CIVORA 0.4 hostile-review runtime proof. Hits the LIVE server over real HTTP.
import json, urllib.request, urllib.error, uuid, sys, time, socket, concurrent.futures

HOST = "http://localhost:8090"
BASE = HOST + "/api/v1"
RUN  = uuid.uuid4().hex[:8]

P = 0; F = 0; FAILED = []
def check(phase, desc, cond, detail=""):
    global P, F
    if cond:
        P += 1; print("  [PASS] " + desc)
    else:
        F += 1; FAILED.append(phase + ": " + desc + " :: " + str(detail)); print("  [FAIL] " + desc + "  >> " + str(detail))

def call(method, path, token=None, body=None, base=BASE, raw=None, retries=5):
    url = path if path.startswith("http") else base + path
    hdr = {"Content-Type": "application/json", "Connection": "close"}
    if token: hdr["Authorization"] = "Bearer " + token
    data = None
    if raw is not None:
        data = raw.encode() if isinstance(raw, str) else raw
    elif body is not None:
        data = json.dumps(body).encode()
    for attempt in range(retries):
        rq = urllib.request.Request(url, data=data, headers=hdr, method=method)
        try:
            with urllib.request.urlopen(rq, timeout=60) as resp:
                txt = resp.read().decode()
                try: return resp.status, json.loads(txt)
                except Exception: return resp.status, txt
        except urllib.error.HTTPError as e:
            txt = e.read().decode()
            try: return e.code, json.loads(txt)
            except Exception: return e.code, txt
        except (ConnectionResetError, socket.error, OSError) as e:
            if attempt < retries - 1:
                time.sleep(0.5 * (attempt + 1))
                continue
            return 0, str(e)
        except Exception as e:
            return 0, str(e)

def D(resp):
    b = resp[1]
    if isinstance(b, dict):
        return b.get("data")
    print("    !! unexpected non-dict body (status %s): %r" % (resp[0], str(b)[:300]))
    return None

def banner(t): print("\n===== " + t + " =====")

# ---------------------------------------------------------------- BOOTSTRAP
banner("BOOTSTRAP (real HTTP, unauthenticated org+register, then login)")
s, r = call("POST", "/organizations", body={"name": "Org Alpha " + RUN, "slug": "org-alpha-" + RUN})
check("boot", "create Org A (unauthenticated self-service)", s == 201, (s, r))
ORG_A = D((s, r))["id"]

s, r = call("POST", "/organizations", body={"name": "Org Beta " + RUN, "slug": "org-beta-" + RUN})
check("boot", "create Org B", s == 201, (s, r))
ORG_B = D((s, r))["id"]

s, r = call("POST", f"/organizations/{ORG_A}/auth/register", body={"email": f"admin.a.{RUN}@alpha.test", "name": "Admin A", "password": "Str0ngPass!234"})
check("boot", "register Org A first user (=>admin)", s == 201, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/auth/register", body={"email": f"staff.a.{RUN}@alpha.test", "name": "Staff A", "password": "Str0ngPass!234"})
check("boot", "register Org A second user (=>staff)", s == 201, (s, r))
s, r = call("POST", f"/organizations/{ORG_B}/auth/register", body={"email": f"admin.b.{RUN}@beta.test", "name": "Admin B", "password": "Str0ngPass!234"})
check("boot", "register Org B first user (=>admin)", s == 201, (s, r))

TOK_A_ADMIN = D(call("POST", f"/organizations/{ORG_A}/auth/login", body={"email": f"admin.a.{RUN}@alpha.test", "password": "Str0ngPass!234"}))["token"]
TOK_A_STAFF = D(call("POST", f"/organizations/{ORG_A}/auth/login", body={"email": f"staff.a.{RUN}@alpha.test", "password": "Str0ngPass!234"}))["token"]
TOK_B_ADMIN = D(call("POST", f"/organizations/{ORG_B}/auth/login", body={"email": f"admin.b.{RUN}@beta.test", "password": "Str0ngPass!234"}))["token"]
check("boot", "login admin_A / staff_A / admin_B -> tokens", all([TOK_A_ADMIN, TOK_A_STAFF, TOK_B_ADMIN]))

# ---------------------------------------------------------------- PHASE 2
banner("PHASE 2: create + publish Emergency Assistance Assessment (5 fields)")
s, r = call("POST", f"/organizations/{ORG_A}/forms", TOK_A_ADMIN, {"key": "emergency_assessment_" + RUN, "name": "Emergency Assistance Assessment", "description": "Intake assessment for emergency assistance"})
check("p2", "create form (admin)", s == 201, (s, r))
FORM_EMG = D((s, r))["id"]
s, r = call("POST", f"/organizations/{ORG_A}/forms/{FORM_EMG}/versions", TOK_A_ADMIN, {})
check("p2", "create version v1+", s == 201 and D((s, r))["version"] >= 1, (s, r))
V_EMG_1 = D((s, r))["id"]

def add_field(form, ver, key, label, ftype, required, validation=None, options=None, order=0, token=TOK_A_ADMIN, org=ORG_A):
    b = {"version_id": ver, "key": key, "label": label, "type": ftype, "required": required, "order": order}
    if validation is not None: b["validation"] = validation
    if options is not None: b["options"] = options
    return call("POST", f"/organizations/{org}/forms/{form}/fields", token, b)

s, r = add_field(FORM_EMG, V_EMG_1, "household_size", "Household Size", "NUMBER", True, {"min_value": 1}, None, 1)
check("p2", "field household_size NUMBER required min_value=1", s == 201, (s, r))
s, r = add_field(FORM_EMG, V_EMG_1, "monthly_income", "Monthly Income", "DECIMAL", True, {"min_value": 0}, None, 2)
check("p2", "field monthly_income DECIMAL required min_value=0", s == 201, (s, r))
s, r = add_field(FORM_EMG, V_EMG_1, "housing_situation", "Housing Situation", "SELECT", True, None,
                 [{"label": "Rent", "value": "rent"}, {"label": "Own", "value": "own"}, {"label": "Shelter", "value": "shelter"}, {"label": "None", "value": "none"}], 3)
check("p2", "field housing_situation SELECT required (4 options)", s == 201, (s, r))
s, r = add_field(FORM_EMG, V_EMG_1, "urgency", "Urgency", "SELECT", True, None,
                 [{"label": "Low", "value": "low"}, {"label": "Medium", "value": "medium"}, {"label": "High", "value": "high"}, {"label": "Critical", "value": "critical"}], 4)
check("p2", "field urgency SELECT required (4 options)", s == 201, (s, r))
s, r = add_field(FORM_EMG, V_EMG_1, "notes", "Notes", "TEXTAREA", False, None, None, 5)
check("p2", "field notes TEXTAREA optional", s == 201, (s, r))

s, r = call("POST", f"/organizations/{ORG_A}/forms/{FORM_EMG}/versions/{V_EMG_1}/publish", TOK_A_ADMIN, {})
check("p2", "publish v1 -> PUBLISHED", s == 200 and D((s, r))["status"] == "PUBLISHED", (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/forms/{FORM_EMG}/active-version", TOK_A_ADMIN)
check("p2", "active-version == v1 PUBLISHED", s == 200 and D((s, r))["status"] == "PUBLISHED", (s, r))
s, r = add_field(FORM_EMG, V_EMG_1, "post_publish_field", "Should Fail", "TEXT", False, None, None, 9)
check("p2", "add field to PUBLISHED version is rejected (immutability)", s in (400, 409), (s, r))

# ---------------------------------------------------------------- PHASE 3
banner("PHASE 3: create + publish a DIFFERENT form Medical Assistance Assessment")
s, r = call("POST", f"/organizations/{ORG_A}/forms", TOK_A_ADMIN, {"key": "medical_assessment_" + RUN, "name": "Medical Assistance Assessment", "description": "Triage and consent"})
check("p3", "create medical form", s == 201, (s, r))
FORM_MED = D((s, r))["id"]
V_MED_1 = D(call("POST", f"/organizations/{ORG_A}/forms/{FORM_MED}/versions", TOK_A_ADMIN, {}))["id"]
add_field(FORM_MED, V_MED_1, "patient_condition", "Patient Condition", "TEXT", True, {"min_length": 3}, None, 1)
add_field(FORM_MED, V_MED_1, "triage_level", "Triage Level", "SELECT", True, None,
          [{"label": "Green", "value": "green"}, {"label": "Yellow", "value": "yellow"}, {"label": "Red", "value": "red"}], 2)
add_field(FORM_MED, V_MED_1, "body_temp", "Body Temperature", "DECIMAL", False, {"min_value": 30, "max_value": 45}, None, 3)
add_field(FORM_MED, V_MED_1, "consent_given", "Consent Given", "BOOLEAN", True, None, None, 4)
s, r = call("POST", f"/organizations/{ORG_A}/forms/{FORM_MED}/versions/{V_MED_1}/publish", TOK_A_ADMIN, {})
check("p3", "publish medical v1 -> PUBLISHED", s == 200 and D((s, r))["status"] == "PUBLISHED", (s, r))

# ---------------------------------------------------------------- PHASE 4
banner("PHASE 4: custom workflow via HTTP + state assignments + cross-org assignment rejection")
wf_body = {
    "key": "hostile_intake_" + RUN, "name": "Hostile Review Intake Workflow", "version": 1, "initial_state": "NEW",
    "states": [
        {"key": "NEW", "name": "New", "terminal": False, "display_order": 0},
        {"key": "INTAKE", "name": "Intake", "terminal": False, "display_order": 1},
        {"key": "ASSESSMENT", "name": "Assessment", "terminal": False, "display_order": 2},
        {"key": "DECISION", "name": "Decision", "terminal": False, "display_order": 3},
        {"key": "CLOSED", "name": "Closed", "terminal": True, "display_order": 4},
    ],
    "transitions": [
        {"key": "start_intake", "name": "Start Intake", "from_state": "NEW", "to_state": "INTAKE", "active": True},
        {"key": "assess", "name": "Assess", "from_state": "INTAKE", "to_state": "ASSESSMENT", "active": True},
        {"key": "decide", "name": "Decide", "from_state": "ASSESSMENT", "to_state": "DECISION", "active": True},
        {"key": "close", "name": "Close", "from_state": "DECISION", "to_state": "CLOSED", "active": True},
    ],
}
s, r = call("POST", f"/organizations/{ORG_A}/workflows", TOK_A_ADMIN, wf_body)
check("p4", "create CUSTOM workflow (novel key/states) -> DRAFT", s == 201 and D((s, r))["status"] == "DRAFT", (s, r))
WF_A = D((s, r))["id"]
s, r = call("POST", f"/organizations/{ORG_A}/workflows/{WF_A}/activate", TOK_A_ADMIN, None)
check("p4", "activate workflow -> ACTIVE", s == 200, (s, r))

s, r = call("POST", f"/organizations/{ORG_A}/workflows/{WF_A}/form-assignments", TOK_A_ADMIN,
            {"workflow_state_key": "NEW", "form_id": FORM_EMG, "form_version_id": V_EMG_1, "required": True, "display_order": 1, "active": True})
check("p4", "assign Emergency v1 REQUIRED -> state NEW", s == 201, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/workflows/{WF_A}/form-assignments", TOK_A_ADMIN,
            {"workflow_state_key": "ASSESSMENT", "form_id": FORM_MED, "form_version_id": V_MED_1, "required": True, "display_order": 1, "active": True})
check("p4", "assign Medical v1 REQUIRED -> state ASSESSMENT", s == 201, (s, r))

FORM_B = D(call("POST", f"/organizations/{ORG_B}/forms", TOK_B_ADMIN, {"key": "beta_form_" + RUN, "name": "Beta Form", "description": "B"}))["id"]
V_B = D(call("POST", f"/organizations/{ORG_B}/forms/{FORM_B}/versions", TOK_B_ADMIN, {}))["id"]
call("POST", f"/organizations/{ORG_B}/forms/{FORM_B}/versions/{V_B}/publish", TOK_B_ADMIN, {})
s, r = call("POST", f"/organizations/{ORG_A}/workflows/{WF_A}/form-assignments", TOK_A_ADMIN,
            {"workflow_state_key": "INTAKE", "form_id": FORM_B, "form_version_id": V_B, "required": True, "display_order": 1, "active": True})
check("p4", "Org A CANNOT assign Org Bs form (cross-tenant)", s in (400, 403, 404), (s, r))
WF_B = D(call("POST", f"/organizations/{ORG_B}/workflows", TOK_B_ADMIN, {
    "key": "beta_wf_" + RUN, "name": "Beta WF", "version": 1, "initial_state": "NEW",
    "states": [{"key": "NEW", "name": "New", "terminal": False, "display_order": 0}, {"key": "DONE", "name": "Done", "terminal": True, "display_order": 1}],
    "transitions": [{"key": "finish", "name": "Finish", "from_state": "NEW", "to_state": "DONE", "active": True}]}))["id"]
call("POST", f"/organizations/{ORG_B}/workflows/{WF_B}/activate", TOK_B_ADMIN, None)
s, r = call("POST", f"/organizations/{ORG_B}/workflows/{WF_B}/form-assignments", TOK_B_ADMIN,
            {"workflow_state_key": "NEW", "form_id": FORM_EMG, "form_version_id": V_EMG_1, "required": True, "display_order": 1, "active": True})
check("p4", "Org B CANNOT assign Org As form (cross-tenant)", s in (400, 403, 404), (s, r))

# ---------------------------------------------------------------- PHASE 5
banner("PHASE 5: case -> instance -> required form -> invalid then valid submission -> persistence")
s, r = call("POST", f"/organizations/{ORG_A}/cases", TOK_A_ADMIN, {"title": "Emergency intake " + RUN, "description": "Family of 4", "service_type": "EMERGENCY", "priority": "HIGH", "workflow_id": WF_A})
check("p5", "create case bound to CUSTOM workflow via workflow_id", s == 201, (s, r))
CASE1 = D((s, r))["id"]
check("p5", "case has workflow_instance_id", bool(D((s, r)).get("workflow_instance_id")), D((s, r)))
s, r = call("GET", f"/organizations/{ORG_A}/cases/{CASE1}/workflow", TOK_A_ADMIN)
check("p5", "instance current_state == NEW", s == 200 and D((s, r))["instance"]["current_state"] == "NEW", (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/cases/{CASE1}/workflow/forms", TOK_A_ADMIN)
check("p5", "GET required forms for NEW lists Emergency form", s == 200 and FORM_EMG in json.dumps(r), (s, r))

s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", TOK_A_ADMIN,
            {"form_version_id": V_EMG_1, "data": {"household_size": 0, "monthly_income": 100, "housing_situation": "rent", "urgency": "high"}})
check("p5", "reject household_size=0 (min_value=1)", s == 400, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", TOK_A_ADMIN,
            {"form_version_id": V_EMG_1, "data": {"household_size": 4, "housing_situation": "rent", "urgency": "high"}})
check("p5", "reject missing required monthly_income", s == 400, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", TOK_A_ADMIN,
            {"form_version_id": V_EMG_1, "data": {"household_size": 4, "monthly_income": 100, "housing_situation": "mansion", "urgency": "high"}})
check("p5", "reject invalid SELECT option housing_situation=mansion", s == 400, (s, r))
VALID = {"household_size": 4, "monthly_income": 2500.50, "housing_situation": "rent", "urgency": "high", "notes": "Verified by caseworker; sensitive note XYZ-9911."}
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", TOK_A_ADMIN, {"form_version_id": V_EMG_1, "data": VALID})
check("p5", "accept VALID submission -> 201", s == 201, (s, r))
SUB1 = D((s, r)).get("id") if isinstance(D((s, r)), dict) else None
s, r = call("GET", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", TOK_A_ADMIN)
blob = json.dumps(r)
check("p5", "reload shows persisted submission for v1", s == 200 and V_EMG_1 in blob, (s, r))
check("p5", "persisted values round-trip", "household_size" in blob and "rent" in blob, blob[:300])
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", TOK_A_ADMIN, {"form_version_id": V_EMG_1, "data": VALID})
check("p5", "duplicate submission for same case+version -> 409", s == 409, (s, r))

# ---------------------------------------------------------------- PHASE 7 (CRITICAL)
banner("PHASE 7: workflow enforcement on BOTH endpoints (backend must reject; not frontend)")
s, r = call("POST", f"/organizations/{ORG_A}/cases", TOK_A_ADMIN, {"title": "Enforcement case " + RUN, "service_type": "EMERGENCY", "priority": "HIGH", "workflow_id": WF_A})
CASE2 = D((s, r))["id"]
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/workflow/transitions/start_intake", TOK_A_ADMIN, {"reason": "try to bypass"})
check("p7", "GENERIC /workflow/transitions/start_intake blocked (required form incomplete) -> 409", s == 409, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/transitions", TOK_A_ADMIN, {"status": "INTAKE"})
check("p7", "CASE-STATUS /transitions blocked (required form incomplete) -> 409", s == 409, (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/cases/{CASE2}/workflow", TOK_A_ADMIN)
check("p7", "state still NEW after blocked transitions (rollback)", D((s, r))["instance"]["current_state"] == "NEW", (s, r))
call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/form-submissions", TOK_A_ADMIN, {"form_version_id": V_EMG_1, "data": VALID})
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/workflow/transitions/start_intake", TOK_A_ADMIN, {"reason": "ok now"})
check("p7", "GENERIC transition allowed after Emergency form submitted -> 200", s == 200, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/workflow/transitions/assess", TOK_A_ADMIN, {})
check("p7", "INTAKE->ASSESSMENT (no required form on INTAKE) -> 200", s == 200, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/workflow/transitions/decide", TOK_A_ADMIN, {})
check("p7", "ASSESSMENT->DECISION blocked (Medical form incomplete) -> 409", s == 409, (s, r))
call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/form-submissions", TOK_A_ADMIN,
     {"form_version_id": V_MED_1, "data": {"patient_condition": "Acute respiratory", "triage_level": "yellow", "consent_given": True}})
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/workflow/transitions/decide", TOK_A_ADMIN, {})
check("p7", "ASSESSMENT->DECISION allowed after Medical form submitted -> 200", s == 200, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE2}/workflow/transitions/close", TOK_A_ADMIN, {})
check("p7", "DECISION->CLOSED -> 200", s == 200, (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/cases/{CASE2}/workflow/history", TOK_A_ADMIN)
hist = D((s, r)) if isinstance(D((s, r)), list) else []
check("p7", "workflow history recorded 4 transitions", len(hist) == 4, [h.get("transition_key") for h in hist])

# ---------------------------------------------------------------- PHASE 6
banner("PHASE 6: versioning - v1 submissions stay pinned to v1 after v2 published")
s, r = call("POST", f"/organizations/{ORG_A}/forms/{FORM_EMG}/versions", TOK_A_ADMIN, {})
check("p6", "create v2 draft version", s == 201, (s, str(r)[:200]))
V_EMG_2 = (D((s, r)) or {}).get("id")
if not V_EMG_2:
    check("p6", "v2 version id extracted", False, "could not extract v2 id")
else:
    add_field(FORM_EMG, V_EMG_2, "household_size", "Household Size", "NUMBER", True, {"min_value": 1}, None, 1)
    add_field(FORM_EMG, V_EMG_2, "monthly_income", "Monthly Income", "DECIMAL", True, {"min_value": 0}, None, 2)
    add_field(FORM_EMG, V_EMG_2, "housing_situation", "Housing Situation", "SELECT", True, None,
              [{"label": "Rent", "value": "rent"}, {"label": "Own", "value": "own"}, {"label": "Shelter", "value": "shelter"}, {"label": "None", "value": "none"}], 3)
    add_field(FORM_EMG, V_EMG_2, "urgency", "Urgency", "SELECT", True, None,
              [{"label": "Low", "value": "low"}, {"label": "Medium", "value": "medium"}, {"label": "High", "value": "high"}, {"label": "Critical", "value": "critical"}], 4)
    add_field(FORM_EMG, V_EMG_2, "v2_only_field", "V2 Field", "TEXT", False, None, None, 5)
    s, r = call("POST", f"/organizations/{ORG_A}/forms/{FORM_EMG}/versions/{V_EMG_2}/publish", TOK_A_ADMIN, {})
    check("p6", "publish v2 -> PUBLISHED", s == 200, (s, r))
    s, r = call("GET", f"/organizations/{ORG_A}/forms/{FORM_EMG}/active-version", TOK_A_ADMIN)
    check("p6", "active-version now v2", s == 200 and D((s, r))["id"] == V_EMG_2, (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", TOK_A_ADMIN)
subs = D((s, r)) if isinstance(D((s, r)), list) else []
pinned = any((x.get("form_version_id") == V_EMG_1) for x in subs) if subs and isinstance(subs[0], dict) else (V_EMG_1 in json.dumps(r))
check("p6", "CASE1 submission STILL pinned to v1 (not migrated to v2)", pinned, json.dumps(r)[:300])
check("p6", "v1 id != v2 id (distinct versions)", V_EMG_1 != V_EMG_2)

# ---------------------------------------------------------------- PHASE 8
banner("PHASE 8: multi-tenant attacks (Org B token vs Org A resources + ID guessing)")
s, r = call("GET", f"/organizations/{ORG_A}/cases", TOK_B_ADMIN)
check("p8", "B token on A path (list cases) -> 403", s == 403, (s, r))
s, r = call("GET", f"/organizations/{ORG_B}/cases/{CASE1}", TOK_B_ADMIN)
check("p8", "As case id under B path -> 404", s == 404, (s, r))
s, r = call("GET", f"/organizations/{ORG_B}/forms/{FORM_EMG}", TOK_B_ADMIN)
check("p8", "As form id under B path -> 404", s == 404, (s, r))
s, r = call("POST", f"/organizations/{ORG_B}/cases/{CASE1}/form-submissions", TOK_B_ADMIN, {"form_version_id": V_EMG_1, "data": VALID})
check("p8", "submit to As case via B path -> 404", s == 404, (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/cases/{CASE1}", TOK_B_ADMIN)
check("p8", "B token read A case on A path -> 403", s == 403, (s, r))
s, r = call("GET", f"/organizations/{ORG_B}/cases/{uuid.uuid4()}", TOK_B_ADMIN)
check("p8", "random guessed case id -> 404", s == 404, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE1}/workflow/transitions/start_intake", TOK_B_ADMIN, {})
check("p8", "B token transition As case -> 403", s == 403, (s, r))

# ---------------------------------------------------------------- PHASE 9
banner("PHASE 9: authorization (staff vs admin, unauthenticated) - server side")
s, r = call("POST", f"/organizations/{ORG_A}/forms", TOK_A_STAFF, {"key": "staff_form_" + RUN, "name": "x", "description": "y"})
check("p9", "staff CANNOT create form (admin-only) -> 403", s == 403, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/workflows/{WF_A}/form-assignments", TOK_A_STAFF,
            {"workflow_state_key": "NEW", "form_id": FORM_EMG, "form_version_id": V_EMG_1, "required": True, "display_order": 1, "active": True})
check("p9", "staff CANNOT create assignment (admin-only) -> 403", s == 403, (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/forms", TOK_A_STAFF)
check("p9", "staff CAN list forms (read) -> 200", s == 200, (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/cases", None)
check("p9", "unauthenticated list cases -> 401", s == 401, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", None, {"form_version_id": V_EMG_1, "data": VALID})
check("p9", "unauthenticated submit -> 401", s == 401, (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/audit", None)
check("p9", "unauthenticated audit -> 401", s == 401, (s, r))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE1}/form-submissions", "totally.invalid.jwt", {"form_version_id": V_EMG_1, "data": VALID})
check("p9", "garbage bearer token submit -> 401", s == 401, (s, r))

# ---------------------------------------------------------------- PHASE 10
banner("PHASE 10: form-data injection / malformed / oversized / wrong-type attacks")
s, r = call("POST", f"/organizations/{ORG_A}/cases", TOK_A_ADMIN, {"title": "Injection case " + RUN, "service_type": "EMERGENCY", "priority": "HIGH", "workflow_id": WF_A})
CASE3 = D((s, r))["id"]
def inj(desc, raw=None, body=None, expect=(400, 413, 409, 422)):
    s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE3}/form-submissions", TOK_A_ADMIN, body=body, raw=raw)
    check("p10", desc, s in expect, (s, str(r)[:160]))
inj("unknown extra field rejected", body={"form_version_id": V_EMG_1, "data": {"household_size": 4, "monthly_income": 1, "housing_situation": "rent", "urgency": "high", "__evil_admin": "x"}})
inj("wrong type household_size=four", body={"form_version_id": V_EMG_1, "data": {"household_size": "four", "monthly_income": 1, "housing_situation": "rent", "urgency": "high"}})
inj("invalid option urgency=apocalyptic", body={"form_version_id": V_EMG_1, "data": {"household_size": 4, "monthly_income": 1, "housing_situation": "rent", "urgency": "apocalyptic"}})
inj("nested object for number (NoSQL-style)", body={"form_version_id": V_EMG_1, "data": {"household_size": {"$gt": 1}, "monthly_income": 1, "housing_situation": "rent", "urgency": "high"}})
inj("NaN literal -> malformed JSON rejected", raw='{"form_version_id": "' + V_EMG_1 + '", "data": {"household_size": NaN, "monthly_income": 1, "housing_situation": "rent", "urgency": "high"}}')
inj("Infinity literal -> rejected", raw='{"form_version_id": "' + V_EMG_1 + '", "data": {"household_size": 4, "monthly_income": Infinity, "housing_situation": "rent", "urgency": "high"}}')
inj("malformed JSON body", raw='{this is not json')
inj("oversized notes (11MB) -> 413 body limit", body={"form_version_id": V_EMG_1, "data": {"household_size": 4, "monthly_income": 1, "housing_situation": "rent", "urgency": "high", "notes": "A" * (11 * 1024 * 1024)}}, expect=(400, 413, 409, 422, 0))
inj("SQL/script string in notes accepted-or-rejected safely (no 500)", body={"form_version_id": V_EMG_1, "data": {"household_size": 4, "monthly_income": 1, "housing_situation": "rent", "urgency": "high", "notes": "'; DROP TABLE cases;-- <script>alert(1)</script>"}}, expect=(201, 400, 409))
s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE3}/form-submissions", TOK_A_ADMIN, {"form_version_id": str(uuid.uuid4()), "data": VALID})
check("p10", "unknown/random form_version_id rejected (not 500)", s in (400, 404, 409), (s, str(r)[:160]))

# ---------------------------------------------------------------- PHASE 11
banner("PHASE 11: concurrency - duplicate submissions race")
s, r = call("POST", f"/organizations/{ORG_A}/cases", TOK_A_ADMIN, {"title": "Concurrency case " + RUN, "service_type": "EMERGENCY", "priority": "HIGH", "workflow_id": WF_A})
CASE_CONC = D((s, r))["id"]
results = []
def submit_concurrent(i):
    s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE_CONC}/form-submissions", TOK_A_ADMIN, {"form_version_id": V_EMG_1, "data": VALID})
    return s
with concurrent.futures.ThreadPoolExecutor(max_workers=5) as ex:
    futs = [ex.submit(submit_concurrent, i) for i in range(5)]
    results = [f.result() for f in futs]
ok_count = results.count(201)
conflict_count = results.count(409)
check("p11", "concurrent race: exactly 1 succeeds (201), rest get 409", ok_count == 1 and conflict_count == 4, results)

# ---------------------------------------------------------------- PHASE 12
banner("PHASE 12: audit - correlation id, hash chain, tenant scope, no sensitive values")
s, r = call("GET", f"/organizations/{ORG_A}/audit?per_page=200", TOK_A_ADMIN)
ev = D((s, r)) if isinstance(D((s, r)), list) else []
check("p12", "audit list -> 200 with events", s == 200 and len(ev) > 0, (s, len(ev)))
subm = [e for e in ev if e.get("action") == "form.submitted"]
check("p12", "form.submitted audit events exist", len(subm) >= 1, [e.get("action") for e in ev][:20])
if subm:
    e0 = subm[0]
    check("p12", "audit event has request_id (correlation)", bool(e0.get("request_id")), e0)
    check("p12", "audit event has hash + previous_hash", bool(e0.get("hash")) and "previous_hash" in e0, e0)
    check("p12", "audit event integrity_valid == true", e0.get("integrity_valid") is True, e0)
    check("p12", "audit event scoped to Org A", e0.get("organization_id") == ORG_A, e0)
check("p12", "ALL returned events integrity_valid", all(e.get("integrity_valid") for e in ev), "some invalid")
check("p12", "ALL returned events belong to Org A (no leakage)", all(e.get("organization_id") == ORG_A for e in ev), "cross-tenant event leaked")
audit_blob = json.dumps(r)
check("p12", "audit API body does NOT leak sensitive note value", "XYZ-9911" not in audit_blob, "leaked")
check("p12", "audit API body does NOT leak monthly_income value", "2500.5" not in audit_blob, "leaked")
s, r = call("GET", f"/organizations/{ORG_A}/audit", TOK_A_STAFF)
check("p12", "staff CAN read audit -> 200", s == 200, (s, r))
s, r = call("GET", f"/organizations/{ORG_A}/audit", TOK_B_ADMIN)
check("p12", "Org B CANNOT read Org A audit -> 403", s == 403, (s, r))

# ---------------------------------------------------------------- PHASE 14
banner("PHASE 14: configurability - novel workflow key + novel fields, ZERO source changes")
check("p14", "custom workflow key not in hardcoded service-type map", not wf_body["key"].startswith(("emergency_", "medical_", "financial_", "food_", "shelter_", "education_", "transport_", "general_")), wf_body["key"])
s, r = call("GET", f"/organizations/{ORG_A}/cases/{CASE2}/workflow", TOK_A_ADMIN)
check("p14", "case on custom workflow reached CLOSED (full custom lifecycle ran)", D((s, r))["instance"]["current_state"] == "CLOSED", (s, r))
resp_x = call("POST", f"/organizations/{ORG_A}/forms", TOK_A_ADMIN, {"key": "contact_" + RUN, "name": "Contact", "description": "c"})
FORM_X = (D(resp_x) or {}).get("id")
if FORM_X:
    resp_vx = call("POST", f"/organizations/{ORG_A}/forms/{FORM_X}/versions", TOK_A_ADMIN, {})
    V_X = (D(resp_vx) or {}).get("id")
    if V_X:
        s1, _ = add_field(FORM_X, V_X, "email", "Email", "EMAIL", True, None, None, 1)
        s2, _ = add_field(FORM_X, V_X, "phone", "Phone", "PHONE", True, None, None, 2)
        s3, _ = add_field(FORM_X, V_X, "intake_date", "Intake Date", "DATE", True, None, None, 3)
        check("p14", "new form w/ EMAIL+PHONE+DATE fields created via HTTP (no source change)", s1 == 201 and s2 == 201 and s3 == 201, (s1, s2, s3))
        call("POST", f"/organizations/{ORG_A}/forms/{FORM_X}/versions/{V_X}/publish", TOK_A_ADMIN, {})
        s, r = call("POST", f"/organizations/{ORG_A}/cases", TOK_A_ADMIN, {"title": "Contact case " + RUN, "service_type": "GENERAL", "priority": "NORMAL", "workflow_id": WF_A})
        CASE4 = D((s, r))["id"]
        s, r = call("POST", f"/organizations/{ORG_A}/cases/{CASE4}/form-submissions", TOK_A_ADMIN, {"form_version_id": V_X, "data": {"email": "not-an-email", "phone": "123", "intake_date": "2026-13-45"}})
        check("p14", "new form validates email/phone/date (rejects bad) -> 400", s == 400, (s, r))
    else:
        check("p14", "v_X version created", False, resp_vx)
else:
    check("p14", "FORM_X created", False, resp_x)

# ---------------------------------------------------------------- SUMMARY
banner("SUMMARY")
print(f"PASS={P}  FAIL={F}")
if FAILED:
    print("\nFAILURES:")
    for f in FAILED: print("  - " + f)
with open("/tmp/civora_proof_ids.json", "w") as fh:
    json.dump({"RUN": RUN, "ORG_A": ORG_A, "ORG_B": ORG_B, "CASE1": CASE1, "CASE2": CASE2, "CASE3": CASE3,
               "V_EMG_1": V_EMG_1, "V_EMG_2": V_EMG_2 if 'V_EMG_2' in dir() else None, "FORM_EMG": FORM_EMG, "WF_A": WF_A}, fh)
print("\nids written to /tmp/civora_proof_ids.json")
sys.exit(1 if F else 0)
