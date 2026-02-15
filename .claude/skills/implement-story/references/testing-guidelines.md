# Testing Guidelines

## Test Location

All tests live in the `OraTests/` directory, mirroring the structure of `Ora/`.

## Naming Conventions

**Test files:** `<Component>Tests.swift`
- `CalendarToolTests.swift`
- `AudioCaptureTests.swift`
- `ConversationManagerTests.swift`

**Test methods:** `test_<feature>_<scenario>_<expectedBehavior>()`
- `test_createEvent_withValidInput_succeeds()`
- `test_createEvent_withMissingTitle_throwsValidationError()`
- `test_transcribe_withEmptyAudio_returnsEmptyString()`

## Test Structure

```swift
import XCTest
@testable import Ora

final class ComponentTests: XCTestCase {
    
    // MARK: - Properties
    
    private var sut: ComponentUnderTest!  // System Under Test
    
    // MARK: - Setup & Teardown
    
    override func setUp() {
        super.setUp()
        sut = ComponentUnderTest()
    }
    
    override func tearDown() {
        sut = nil
        super.tearDown()
    }
    
    // MARK: - Happy Path Tests
    
    func test_operation_withValidInput_succeeds() {
        // Given
        let input = ValidInput()
        
        // When
        let result = sut.perform(input)
        
        // Then
        XCTAssertEqual(result, expectedValue)
    }
    
    // MARK: - Error Cases
    
    func test_operation_withInvalidInput_throwsError() {
        // Given
        let invalidInput = InvalidInput()
        
        // Then
        XCTAssertThrowsError(try sut.perform(invalidInput)) { error in
            XCTAssertEqual(error as? ComponentError, .invalidInput)
        }
    }
    
    // MARK: - Edge Cases
    
    func test_operation_withEmptyInput_handlesGracefully() {
        // Given
        let emptyInput = ""
        
        // When
        let result = sut.perform(emptyInput)
        
        // Then
        XCTAssertNil(result)
    }
}
```

## What to Test by Component Type

| Component Type | Test Focus |
|:---------------|:-----------|
| **Tools** | Input validation, execution success/failure, error handling, EventKit/Contacts integration |
| **Services** | State transitions, async behavior, cancellation, error propagation |
| **UI ViewModels** | State changes on user actions, published property updates, error states |
| **Utilities** | Pure functions, edge cases, parsing, formatting |
| **Actors** | Concurrent access, isolation, message ordering |

## Running Tests

```bash
# Full test suite
xcodebuild test -project Ora.xcodeproj -scheme Ora

# With Thread Sanitizer (detect concurrency issues)
xcodebuild test -project Ora.xcodeproj -scheme Ora-TSan

# Specific test class (faster iteration)
xcodebuild test -project Ora.xcodeproj -scheme Ora \
  -only-testing:OraTests/CalendarToolTests

# Specific test method
xcodebuild test -project Ora.xcodeproj -scheme Ora \
  -only-testing:OraTests/CalendarToolTests/test_createEvent_withValidInput_succeeds
```

## Coverage Requirements

- **Target:** ≥85% coverage for new code
- **Critical paths:** 100% coverage for security-sensitive and data-mutation code
- **If target not achievable:** Document gaps explicitly in story file with justification

## Async Testing

```swift
// Modern async/await syntax
func test_asyncOperation_completes() async throws {
    // Given
    let service = AsyncService()
    
    // When
    let result = try await service.performOperation()
    
    // Then
    XCTAssertEqual(result.status, .success)
}

// With timeout
func test_asyncOperation_completesWithinTimeout() async throws {
    let result = try await withTimeout(seconds: 5) {
        try await service.longRunningOperation()
    }
    XCTAssertNotNil(result)
}
```

## Test Fixtures

Place test data in `OraTests/Fixtures/`:

```
OraTests/
├── Fixtures/
│   ├── valid-event.json
│   ├── malformed-response.json
│   ├── sample-audio.wav
│   └── contacts-response.json
└── ...
```

Load fixtures in tests:

```swift
func loadFixture(_ name: String) throws -> Data {
    let bundle = Bundle(for: type(of: self))
    guard let url = bundle.url(forResource: name, withExtension: nil, subdirectory: "Fixtures") else {
        throw FixtureError.notFound(name)
    }
    return try Data(contentsOf: url)
}
```

## Mocking

For external dependencies, create protocol-based mocks:

```swift
// Protocol
protocol EventStoreProviding {
    func events(matching predicate: NSPredicate) -> [EKEvent]
}

// Production implementation
extension EKEventStore: EventStoreProviding {}

// Mock for testing
class MockEventStore: EventStoreProviding {
    var eventsToReturn: [EKEvent] = []
    
    func events(matching predicate: NSPredicate) -> [EKEvent] {
        return eventsToReturn
    }
}
```

## Before Handoff Checklist

- [ ] All new code has corresponding tests
- [ ] Tests cover happy path
- [ ] Tests cover error/edge cases
- [ ] All tests pass locally
- [ ] No flaky tests introduced
- [ ] Thread Sanitizer run shows no issues
- [ ] Coverage target met (or gaps documented)
