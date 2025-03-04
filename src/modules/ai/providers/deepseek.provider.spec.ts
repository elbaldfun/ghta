import { Test, TestingModule } from '@nestjs/testing';
import { ConfigService } from '@nestjs/config';
import { DeepseekProvider } from './deepseek.provider';
import { OpenAI } from 'openai';

// Mock OpenAI
jest.mock('openai', () => {
  return {
    OpenAI: jest.fn().mockImplementation(() => ({
      chat: {
        completions: {
          create: jest.fn(),
        },
      },
    })),
  };
});

describe('DeepseekProvider', () => {
  let provider: DeepseekProvider;
  let configService: ConfigService;
  let openaiMock: jest.Mocked<OpenAI>;

  beforeEach(async () => {
    // Reset mocks before each test
    jest.clearAllMocks();

    // Create a mock ConfigService
    const configServiceMock = {
      get: jest.fn((key: string) => {
        if (key === 'LMSTUDIO_BASE_URL') {
          return 'http://192.168.50.206:1234/v1';
        }
        return undefined;
      }),
    };

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeepseekProvider,
        {
          provide: ConfigService,
          useValue: configServiceMock,
        },
      ],
    }).compile();

    provider = module.get<DeepseekProvider>(DeepseekProvider);
    configService = module.get<ConfigService>(ConfigService);
    openaiMock = new OpenAI() as jest.Mocked<OpenAI>;
  });

  it('should be defined', () => {
    expect(provider).toBeDefined();
  });

  it('should initialize OpenAI with correct configuration', () => {
    // Verify OpenAI was constructed with the correct parameters
    expect(OpenAI).toHaveBeenCalledWith({
      baseURL: 'http://192.168.50.206:1234/v1',
      apiKey: 'not-needed',
    });
  });

  it('should use default URL if LMSTUDIO_BASE_URL is not provided', async () => {
    // Mock configService.get to return undefined for LMSTUDIO_BASE_URL
    jest.spyOn(configService, 'get').mockImplementation((key: string) => {
      if (key === 'LMSTUDIO_BASE_URL') {
        return undefined;
      }
      return undefined;
    });

    // Re-create the provider to trigger the constructor with the new mock
    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeepseekProvider,
        {
          provide: ConfigService,
          useValue: configService,
        },
      ],
    }).compile();

    // Get the new provider instance
    const newProvider = module.get<DeepseekProvider>(DeepseekProvider);
    expect(newProvider).toBeDefined();

    // Verify OpenAI was constructed with the default URL
    expect(OpenAI).toHaveBeenCalledWith({
      baseURL: 'http://localhost:1234/v1',
      apiKey: 'not-needed',
    });
  });

  describe('analyze', () => {
    it('should return the model response when successful', async () => {
      // Arrange
      const mockPrompt = 'Analyze this GitHub repository: https://github.com/example/repo';
      const mockResponse = 'This repository is a JavaScript framework for web development.';
      
      // Mock the OpenAI chat.completions.create method
      const createMock = jest.fn().mockResolvedValue({
        choices: [
          {
            message: {
              content: mockResponse,
            },
          },
        ],
      });

      // Replace the implementation of OpenAI's create method
      // Cast to unknown first to avoid TypeScript errors
      ((OpenAI as unknown) as jest.Mock).mockImplementation(() => ({
        chat: {
          completions: {
            create: createMock,
          },
        },
      }));

      // Re-create the provider to use our new mock
      const module: TestingModule = await Test.createTestingModule({
        providers: [
          DeepseekProvider,
          {
            provide: ConfigService,
            useValue: configService,
          },
        ],
      }).compile();

      const providerWithMock = module.get<DeepseekProvider>(DeepseekProvider);

      // Act
      const result = await providerWithMock.analyze(mockPrompt);

      // Assert
      expect(createMock).toHaveBeenCalledWith({
        model: 'deepseek-coder',
        messages: [
          {
            role: "system",
            content: "You are a technical expert who categorizes GitHub repositories based on their content and purpose. Please analyze the repository and provide accurate categorization."
          },
          {
            role: "user",
            content: mockPrompt
          }
        ],
        temperature: 0.2,
        max_tokens: 1000,
      });
      expect(result).toBe(mockResponse);
    });

    it('should throw an error when OpenAI API call fails', async () => {
      // Arrange
      const mockPrompt = 'Analyze this GitHub repository: https://github.com/example/repo';
      const mockError = new Error('API connection failed');
      
      // Mock the OpenAI chat.completions.create method to throw an error
      const createMock = jest.fn().mockRejectedValue(mockError);

      // Replace the implementation of OpenAI's create method
      // Cast to unknown first to avoid TypeScript errors
      ((OpenAI as unknown) as jest.Mock).mockImplementation(() => ({
        chat: {
          completions: {
            create: createMock,
          },
        },
      }));

      // Re-create the provider to use our new mock
      const module: TestingModule = await Test.createTestingModule({
        providers: [
          DeepseekProvider,
          {
            provide: ConfigService,
            useValue: configService,
          },
        ],
      }).compile();

      const providerWithMock = module.get<DeepseekProvider>(DeepseekProvider);

      // Act & Assert
      await expect(providerWithMock.analyze(mockPrompt)).rejects.toThrow(
        `Failed to get response from DeepSeek: ${mockError.message}`
      );
    });
  });
}); 