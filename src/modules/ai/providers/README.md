# DeepseekProvider Tests

This directory contains tests for the AI providers used in the GitHub Trend API Service.

## DeepseekProvider Test

The `deepseek.provider.spec.ts` file contains unit tests for the `DeepseekProvider` class, which is responsible for interacting with the DeepSeek model through LM Studio's API.

### Test Coverage

The tests cover:

1. Initialization of the OpenAI client with the correct configuration
2. Handling of configuration values (using default URL if not provided)
3. Successful analysis of GitHub repositories
4. Error handling when the API call fails

### Running the Tests

To run the tests for the DeepseekProvider specifically, use the following command:

```bash
# From the project root
npm test -- src/modules/ai/providers/deepseek.provider.spec.ts

# Or with pnpm (recommended for this project)
pnpm test -- src/modules/ai/providers/deepseek.provider.spec.ts
```

To run the tests with coverage:

```bash
# From the project root
pnpm test:cov -- src/modules/ai/providers/deepseek.provider.spec.ts
```

### Test Environment

The tests use Jest's mocking capabilities to mock the OpenAI client, so no actual API calls are made during testing. This means:

1. You don't need a running LM Studio instance to run the tests
2. No actual API calls are made to the DeepSeek model
3. All external dependencies are properly mocked

### Notes

- The tests assume that the DeepseekProvider is using the OpenAI SDK to communicate with LM Studio's API
- The configuration is mocked to use the IP address `http://192.168.50.206:1234/v1` as specified in the requirements
- The tests verify that the provider falls back to `http://localhost:1234/v1` if no configuration is provided 