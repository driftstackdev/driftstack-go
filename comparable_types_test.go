package driftstack

// A type that stops being comparable breaks every program that compares two
// values of it with == or !=, uses it as a map key or passes it as a
// comparable type argument, and a diff of exported names cannot see it: adding
// one slice field to a struct changes no name. That happened once already. A
// release note said a version "builds and runs unchanged" while AgentIntent,
// AgentIntentResult and AgentStepEvent had stopped being comparable, because
// AgentIntent gained Frame ([]int); an exported-name diff of the published
// module against the source showed nothing removed.
//
// So this test type-checks the package with go/types and compares it with
// the types that were comparable in the last published release. Each one that
// is no longer comparable, or no longer exists, must be named (in backticks)
// in a Migration note in CHANGELOG.md above that release, which is where a
// program's author looks for what to change.

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

// lastPublishedRelease is the release comparableAtLastPublishedRelease was
// measured on.
const lastPublishedRelease = "0.5.0"

// comparableAtLastPublishedRelease is every exported type that was comparable
// in the module as published at v0.5.0: the proxy.golang.org zip,
// type-checked with go/types on 2026-10-02, 99 of its 190 exported types.
// Re-measure it after each release: `go test -run TestComparableTypes -v`
// logs the list for the current source.
var comparableAtLastPublishedRelease = []string{
	"APIKeyRevokedData", "Account", "AccountProxyExitObserved", "AccountProxyMetadata",
	"AccountProxyOsFingerprint", "AccountProxyTestResult", "AccountProxyVpnConfig",
	"AccountResource", "AccountStatus", "AccountTeamMembership", "AccountTier",
	"AgentFailureDiagnosis", "AgentIntent", "AgentIntentResult", "AgentSessionEgressResult",
	"AgentSessionErrorEvent", "AgentSessionsResource", "AgentStepEvent", "AgentStepWarning",
	"AgentUsage", "ArchetypesResource", "BehavioralProfile", "BundledLlmStatus", "CaptureKind",
	"CaptureRequest", "CaptureResponse", "CaptureSnapshotRequest", "Client",
	"CloneProfileRequest", "ConsequentialActionApproval", "CreateAgentSessionRequest",
	"CreateOptions", "CreateProxyOptions", "CreateRecipeRequest", "EgressResource",
	"ImportProfileRequest", "InteractAction", "InteractRequest", "InteractResponse",
	"LaunchProfileRequest", "ListAgentSessionsQuery", "ListDeliveriesQuery",
	"ListFieldExtraction", "ListProfileSnapshotsQuery", "ListProfilesQuery", "ListRecipesQuery",
	"ListSessionsQuery", "ListSupportConversationsParams", "LiveKitInfo", "NavigateRequest",
	"NavigateResponse", "PageState", "PageStateError", "PendingAcceptance",
	"ProfileActivityEntry", "ProfileExportEnvelope", "ProfileExportPayload",
	"ProfileGeolocation", "ProfileSnapshot", "ProfileSnapshotsResource", "ProfilesResource",
	"PublicArchetype", "RateLimitBucket", "Recipe", "RecipeSuggestion", "RecipesResource",
	"RestoreSnapshotRequest", "ResumeAgentSessionRequest", "ResumeAgentSessionResponse",
	"RetryConfig", "RotateWebhookSecretResponse", "SearchRequest", "SearchResponse",
	"SendTestWebhookResponse", "SessionCompletedData", "SessionGeolocation", "SessionLiveness",
	"SessionLoginRequest", "SessionLoginResponse", "SessionPurpose", "SessionStatus",
	"SessionsResource", "StopAgentTurnResponse", "SupportConversation", "SupportResource",
	"TranscriptOptions", "TrimProfileResponse", "UpdateWebhookRequest", "UsageRecordType",
	"UsageResource", "VerifyWebhookOptions", "WaitCondition", "WaitRequest", "WaitResponse",
	"WebhookDelivery", "WebhookDeliveryStatus", "WebhookEndpointDeliveryCounts",
	"WebhookEventType", "WebhooksResource",
}

// migrationHeading is a CHANGELOG section that tells a program's author what
// to change.
var migrationHeading = regexp.MustCompile(`^### (Migration|Upgrading|Changed — BREAKING)\b`)

var (
	thisPackageOnce sync.Once
	thisPackage     *types.Package
	thisPackageErr  error
)

// typeCheckThisPackage type-checks the package's non-test files, as a program
// that imports the module sees them.
func typeCheckThisPackage(t *testing.T) *types.Package {
	t.Helper()
	thisPackageOnce.Do(func() {
		fset := token.NewFileSet()
		entries, err := os.ReadDir(".")
		if err != nil {
			thisPackageErr = err
			return
		}
		var files []*ast.File
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, name, nil, 0)
			if err != nil {
				thisPackageErr = err
				return
			}
			files = append(files, f)
		}
		conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
		thisPackage, thisPackageErr = conf.Check("driftstack", fset, files, nil)
	})
	if thisPackageErr != nil {
		t.Fatalf("type-checking the package: %v", thisPackageErr)
	}
	return thisPackage
}

// exportedTypes is every exported type name the package declares, sorted.
func exportedTypes(pkg *types.Package) []string {
	var names []string
	for _, name := range pkg.Scope().Names() {
		if tn, ok := pkg.Scope().Lookup(name).(*types.TypeName); ok && tn.Exported() {
			names = append(names, name)
		}
	}
	return names
}

func isComparable(pkg *types.Package, name string) bool {
	tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)
	return ok && types.Comparable(tn.Type())
}

// lostComparability is each name in baseline that is not a comparable type in
// pkg: it stopped being comparable, or it is gone.
func lostComparability(pkg *types.Package, baseline []string) []string {
	var lost []string
	for _, name := range baseline {
		if !isComparable(pkg, name) {
			lost = append(lost, name)
		}
	}
	return lost
}

// migrationNotesAbove is the text of every Migration, Upgrading and
// "Changed — BREAKING" section in changelog above `## [release]`: the notes for
// [Unreleased] and for every release after that one.
func migrationNotesAbove(changelog, release string) (string, bool) {
	end := strings.Index(changelog, "\n## ["+release+"]")
	if end < 0 {
		return "", false
	}
	var notes strings.Builder
	in := false
	for _, line := range strings.Split(changelog[:end], "\n") {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			in = migrationHeading.MatchString(line)
			continue
		}
		if in {
			notes.WriteString(line)
			notes.WriteByte('\n')
		}
	}
	return notes.String(), true
}

func TestComparableTypesStayComparableOrAMigrationNoteNamesThem(t *testing.T) {
	pkg := typeCheckThisPackage(t)

	// Positive controls: the package was really read, and the check tells a
	// comparable type from one that is not.
	if n := len(exportedTypes(pkg)); n < 150 {
		t.Fatalf("read %d exported types; the package has well over 150, so the type-check saw too little", n)
	}
	if !isComparable(pkg, "Client") {
		t.Fatalf("Client is comparable (pointers and strings only); the check cannot tell")
	}
	if isComparable(pkg, "AgentMessageResponse") {
		t.Fatalf("AgentMessageResponse holds slices and is not comparable; the check cannot tell")
	}

	raw, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatalf("reading CHANGELOG.md: %v", err)
	}
	notes, ok := migrationNotesAbove(string(raw), lastPublishedRelease)
	if !ok {
		t.Fatalf("CHANGELOG.md has no ## [%s] heading to measure from", lastPublishedRelease)
	}
	for _, name := range lostComparability(pkg, comparableAtLastPublishedRelease) {
		if !strings.Contains(notes, "`"+name+"`") {
			t.Errorf("%s was comparable in v%s and is not now (or is gone): a program that compares two with == or !=, "+
				"uses one as a map key or passes one as a comparable type argument stops compiling. Name `%s` in a "+
				"### Migration section above ## [%s] in CHANGELOG.md, with what to do instead.",
				name, lastPublishedRelease, name, lastPublishedRelease)
		}
	}

	var now []string
	for _, name := range exportedTypes(pkg) {
		if isComparable(pkg, name) {
			now = append(now, name)
		}
	}
	t.Logf("comparable now (%d): %s", len(now), strings.Join(now, " "))
}

func TestComparableTypesCheckFindsALossAndReadsOnlyTheNotesAboveTheRelease(t *testing.T) {
	pkg := typeCheckThisPackage(t)
	lost := lostComparability(pkg, []string{"Client", "AgentMessageResponse", "NoSuchTypeAnyMore"})
	sort.Strings(lost)
	if want := []string{"AgentMessageResponse", "NoSuchTypeAnyMore"}; !reflect.DeepEqual(lost, want) {
		t.Errorf("lostComparability = %v, want %v (a non-comparable type and a gone one)", lost, want)
	}

	changelog := strings.Join([]string{
		"## [Unreleased]", "", "### Added", "", "- `InAdded`", "",
		"### Migration (2026-10-01)", "", "- `InMigration`", "",
		"### Changed — BREAKING", "", "- `InBreaking`", "",
		"## [0.6.0] - 2026-10-02", "", "### Upgrading from v0.5.0", "", "- `InUpgrading`", "",
		"## [0.5.0] - 2026-09-30", "", "### Migration", "", "- `BelowTheRelease`", "",
	}, "\n")
	notes, ok := migrationNotesAbove(changelog, "0.5.0")
	if !ok {
		t.Fatalf("the release heading was not found")
	}
	for _, in := range []string{"`InMigration`", "`InBreaking`", "`InUpgrading`"} {
		if !strings.Contains(notes, in) {
			t.Errorf("notes miss %s:\n%s", in, notes)
		}
	}
	for _, out := range []string{"`InAdded`", "`BelowTheRelease`"} {
		if strings.Contains(notes, out) {
			t.Errorf("notes include %s, which is not a note above the release:\n%s", out, notes)
		}
	}
	if _, ok := migrationNotesAbove(changelog, "0.4.0"); ok {
		t.Errorf("a release the CHANGELOG does not have was found")
	}
}
