import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { OpenAI } from 'openai';
import { IAiProvider } from '../interfaces/ai-provider.interface';

@Injectable()
export class DeepseekProvider implements IAiProvider {
  private readonly openai: OpenAI;
  private readonly logger = new Logger(DeepseekProvider.name);

  constructor(private configService: ConfigService) {
    // LM Studio 默认运行在 http://localhost:1234
    this.openai = new OpenAI({
      baseURL: this.configService.get<string>('LMSTUDIO_BASE_URL') || 'http://localhost:1234/v1',
      apiKey: 'not-needed', // LM Studio 本地服务不需要 API key
    });
  }

  async analyze(prompt: string): Promise<string> {
    try {
      const completion = await this.openai.chat.completions.create({
        model: this.configService.get<string>('LMSTUDIO_LOCAL_MODULE_NAME'), // 这里使用实际的模型名称
        messages: [
          {
            role: "system",
            content: "You are a technical expert who categorizes GitHub repositories based on their content and purpose. Please analyze the repository and provide accurate categorization."
          },
          {
            role: "user",
            content: prompt
          }
        ],
        temperature: 0.2,
        max_tokens: 1000,
      });

      return completion.choices[0].message.content;
    } catch (error) {
      this.logger.error(`DeepSeek analysis failed: ${error.message}`);
      throw new Error(`Failed to get response from DeepSeek: ${error.message}`);
    }
  }
} 