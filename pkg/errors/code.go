package errors

// ErrorCode represents a unique error identifier
type ErrorCode int

// Error code ranges allocation:
// 10000-10999: System & Common errors
// 11000-11999: User module errors
// 12000-12999: Problem module errors
// 13000-13999: Submission & Judge module errors
// 14000-14999: Contest module errors
// 15000-15999: Discussion & Community errors
// 16000-16999: Admin & Permission errors

const (
	// ========== System & Common Errors (10000-10999) ==========

	// Success
	Success ErrorCode = 10000

	// Generic errors (10000-10099)
	InternalServerError ErrorCode = 10001
	InvalidParams       ErrorCode = 10002
	NotFound            ErrorCode = 10003
	Unauthorized        ErrorCode = 10004
	Forbidden           ErrorCode = 10005
	TooManyRequests     ErrorCode = 10006
	ServiceUnavailable  ErrorCode = 10007
	Timeout             ErrorCode = 10008

	// Database errors (10100-10199)
	DatabaseError       ErrorCode = 10100
	RecordNotFound      ErrorCode = 10101
	RecordAlreadyExists ErrorCode = 10102
	TransactionFailed   ErrorCode = 10103

	// Cache errors (10200-10299)
	CacheError     ErrorCode = 10200
	CacheMiss      ErrorCode = 10201
	CacheSetFailed ErrorCode = 10202
	LockFailed     ErrorCode = 10203

	// Validation errors (10300-10399)
	ValidationFailed   ErrorCode = 10300
	InvalidFormat      ErrorCode = 10301
	InvalidValue       ErrorCode = 10302
	RequiredFieldEmpty ErrorCode = 10303

	// ========== User Module Errors (11000-11999) ==========

	// Authentication (11000-11099)
	InvalidCredentials    ErrorCode = 11000
	UserNotFound          ErrorCode = 11001
	PasswordIncorrect     ErrorCode = 11002
	TokenExpired          ErrorCode = 11003
	TokenInvalid          ErrorCode = 11004
	TokenGenerationFailed ErrorCode = 11005

	// Registration (11100-11199)
	UsernameAlreadyExists ErrorCode = 11100
	EmailAlreadyExists    ErrorCode = 11101
	InvalidUsername       ErrorCode = 11102
	InvalidEmail          ErrorCode = 11103
	InvalidPassword       ErrorCode = 11104
	PasswordTooWeak       ErrorCode = 11105

	// User operations (11200-11299)
	UserUpdateFailed    ErrorCode = 11200
	UserDeleteFailed    ErrorCode = 11201
	InsufficientScore   ErrorCode = 11202
	AccountSuspended    ErrorCode = 11203
	AccountNotActivated ErrorCode = 11204

	// ========== Problem Module Errors (12000-12999) ==========

	// Problem basic (12000-12099)
	ProblemNotFound     ErrorCode = 12000
	ProblemAccessDenied ErrorCode = 12001
	ProblemCreateFailed ErrorCode = 12002
	ProblemUpdateFailed ErrorCode = 12003
	ProblemDeleteFailed ErrorCode = 12004
	ProblemNotPublished ErrorCode = 12005

	// Test cases (12100-12199)
	TestCaseNotFound     ErrorCode = 12100
	TestCaseUploadFailed ErrorCode = 12101
	TestCaseInvalid      ErrorCode = 12102
	TestCaseTooLarge     ErrorCode = 12103

	// Tags & Categories (12200-12299)
	TagNotFound ErrorCode = 12200
	InvalidTag  ErrorCode = 12201
	TooManyTags ErrorCode = 12202

	// ========== Submission & Judge Module Errors (13000-13999) ==========

	// Submission (13000-13099)
	SubmissionNotFound     ErrorCode = 13000
	SubmissionCreateFailed ErrorCode = 13001
	CodeTooLarge           ErrorCode = 13002
	LanguageNotSupported   ErrorCode = 13003
	SubmitTooFrequently    ErrorCode = 13004
	ProblemNotSubmittable  ErrorCode = 13005

	// Judge (13100-13199)
	JudgeQueueFull      ErrorCode = 13100
	JudgeSystemError    ErrorCode = 13101
	CompilationError    ErrorCode = 13102
	RuntimeError        ErrorCode = 13103
	TimeLimitExceeded   ErrorCode = 13104
	MemoryLimitExceeded ErrorCode = 13105
	OutputLimitExceeded ErrorCode = 13106

	// Custom test (13200-13299)
	CustomTestFailed    ErrorCode = 13200
	CustomInputTooLarge ErrorCode = 13201

	// ========== Contest Module Errors (14000-14999) ==========

	// Contest basic (14000-14099)
	ContestNotFound     ErrorCode = 14000
	ContestNotStarted   ErrorCode = 14001
	ContestEnded        ErrorCode = 14002
	ContestAccessDenied ErrorCode = 14003
	ContestCreateFailed ErrorCode = 14004
	ContestUpdateFailed ErrorCode = 14005

	// Registration (14100-14199)
	RegistrationClosed     ErrorCode = 14100
	AlreadyRegistered      ErrorCode = 14101
	RegistrationFailed     ErrorCode = 14102
	NotRegistered          ErrorCode = 14103
	RegistrationNotStarted ErrorCode = 14104

	// Ranking (14200-14299)
	RankingNotAvailable ErrorCode = 14200
	RankingFrozen       ErrorCode = 14201

	// ========== Discussion & Community Errors (15000-15999) ==========

	// Discussion (15000-15099)
	DiscussionNotFound     ErrorCode = 15000
	DiscussionCreateFailed ErrorCode = 15001
	DiscussionUpdateFailed ErrorCode = 15002
	DiscussionDeleteFailed ErrorCode = 15003

	// Comments (15100-15199)
	CommentNotFound     ErrorCode = 15100
	CommentCreateFailed ErrorCode = 15101
	CommentDeleteFailed ErrorCode = 15102

	// ========== Admin & Permission Errors (16000-16999) ==========

	// Permission (16000-16099)
	PermissionDenied       ErrorCode = 16000
	InsufficientPermission ErrorCode = 16001
	RoleNotFound           ErrorCode = 16002
	InvalidRole            ErrorCode = 16003

	// Admin operations (16100-16199)
	AdminOperationFailed ErrorCode = 16100
	BanUserFailed        ErrorCode = 16101
	UnbanUserFailed      ErrorCode = 16102
)

// errorMessages maps error codes to their default English messages
var errorMessages = map[ErrorCode]string{
	// System & Common
	Success:             "Success",
	InternalServerError: "Internal server error",
	InvalidParams:       "Invalid parameters",
	NotFound:            "Resource not found",
	Unauthorized:        "Unauthorized access",
	Forbidden:           "Access forbidden",
	TooManyRequests:     "Too many requests, please try again later",
	ServiceUnavailable:  "Service temporarily unavailable",
	Timeout:             "Request timeout",

	// Database
	DatabaseError:       "Database operation failed",
	RecordNotFound:      "Record not found in database",
	RecordAlreadyExists: "Record already exists",
	TransactionFailed:   "Database transaction failed",

	// Cache
	CacheError:     "Cache operation failed",
	CacheMiss:      "Cache miss",
	CacheSetFailed: "Failed to set cache",
	LockFailed:     "Failed to acquire lock",

	// Validation
	ValidationFailed:   "Validation failed",
	InvalidFormat:      "Invalid format",
	InvalidValue:       "Invalid value",
	RequiredFieldEmpty: "Required field is empty",

	// User - Authentication
	InvalidCredentials:    "Invalid username or password",
	UserNotFound:          "User not found",
	PasswordIncorrect:     "Incorrect password",
	TokenExpired:          "Token has expired",
	TokenInvalid:          "Invalid token",
	TokenGenerationFailed: "Failed to generate token",

	// User - Registration
	UsernameAlreadyExists: "Username already exists",
	EmailAlreadyExists:    "Email already exists",
	InvalidUsername:       "Invalid username format",
	InvalidEmail:          "Invalid email format",
	InvalidPassword:       "Invalid password format",
	PasswordTooWeak:       "Password is too weak",

	// User - Operations
	UserUpdateFailed:    "Failed to update user",
	UserDeleteFailed:    "Failed to delete user",
	InsufficientScore:   "Insufficient score",
	AccountSuspended:    "Account has been suspended",
	AccountNotActivated: "Account is not activated",

	// Problem
	ProblemNotFound:     "Problem not found",
	ProblemAccessDenied: "Access to this problem is denied",
	ProblemCreateFailed: "Failed to create problem",
	ProblemUpdateFailed: "Failed to update problem",
	ProblemDeleteFailed: "Failed to delete problem",
	ProblemNotPublished: "Problem is not published yet",

	// Test cases
	TestCaseNotFound:     "Test case not found",
	TestCaseUploadFailed: "Failed to upload test case",
	TestCaseInvalid:      "Invalid test case format",
	TestCaseTooLarge:     "Test case file is too large",

	// Tags
	TagNotFound: "Tag not found",
	InvalidTag:  "Invalid tag",
	TooManyTags: "Too many tags",

	// Submission
	SubmissionNotFound:     "Submission not found",
	SubmissionCreateFailed: "Failed to create submission",
	CodeTooLarge:           "Code is too large",
	LanguageNotSupported:   "Programming language not supported",
	SubmitTooFrequently:    "Submitting too frequently, please wait",
	ProblemNotSubmittable:  "This problem cannot be submitted at the moment",

	// Judge
	JudgeQueueFull:      "Judge queue is full, please try again later",
	JudgeSystemError:    "Judge system error",
	CompilationError:    "Compilation error",
	RuntimeError:        "Runtime error",
	TimeLimitExceeded:   "Time limit exceeded",
	MemoryLimitExceeded: "Memory limit exceeded",
	OutputLimitExceeded: "Output limit exceeded",

	// Custom test
	CustomTestFailed:    "Custom test execution failed",
	CustomInputTooLarge: "Custom input is too large",

	// Contest
	ContestNotFound:     "Contest not found",
	ContestNotStarted:   "Contest has not started yet",
	ContestEnded:        "Contest has ended",
	ContestAccessDenied: "Access to this contest is denied",
	ContestCreateFailed: "Failed to create contest",
	ContestUpdateFailed: "Failed to update contest",

	// Contest Registration
	RegistrationClosed:     "Registration is closed",
	AlreadyRegistered:      "Already registered for this contest",
	RegistrationFailed:     "Registration failed",
	NotRegistered:          "Not registered for this contest",
	RegistrationNotStarted: "Registration has not started yet",

	// Ranking
	RankingNotAvailable: "Ranking is not available",
	RankingFrozen:       "Ranking is frozen",

	// Discussion
	DiscussionNotFound:     "Discussion not found",
	DiscussionCreateFailed: "Failed to create discussion",
	DiscussionUpdateFailed: "Failed to update discussion",
	DiscussionDeleteFailed: "Failed to delete discussion",

	// Comments
	CommentNotFound:     "Comment not found",
	CommentCreateFailed: "Failed to create comment",
	CommentDeleteFailed: "Failed to delete comment",

	// Permission
	PermissionDenied:       "Permission denied",
	InsufficientPermission: "Insufficient permission",
	RoleNotFound:           "Role not found",
	InvalidRole:            "Invalid role",

	// Admin
	AdminOperationFailed: "Admin operation failed",
	BanUserFailed:        "Failed to ban user",
	UnbanUserFailed:      "Failed to unban user",
}

// Message returns the default message for the error code
func (c ErrorCode) Message() string {
	if msg, ok := errorMessages[c]; ok {
		return msg
	}
	return "Unknown error"
}

// HTTPStatus returns the recommended HTTP status code for the error code
func (c ErrorCode) HTTPStatus() int {
	switch {
	case c == Success:
		return 200
	case c >= 11000 && c < 11100: // Authentication errors
		return 401
	case c == Unauthorized, c == TokenExpired, c == TokenInvalid:
		return 401
	case c == Forbidden, c >= 16000 && c < 16100: // Permission errors
		return 403
	case c == NotFound, c == UserNotFound, c == ProblemNotFound, c == ContestNotFound:
		return 404
	case c == TooManyRequests, c == SubmitTooFrequently:
		return 429
	case c == ServiceUnavailable:
		return 503
	case c >= 10300 && c < 10400: // Validation errors
		return 400
	case c == InvalidParams:
		return 400
	default:
		return 500
	}
}
