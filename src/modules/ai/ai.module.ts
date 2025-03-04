import { Module } from '@nestjs/common';
import { MongooseModule } from '@nestjs/mongoose';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { AiService } from './ai.service';
import { OpenAiProvider } from './providers/openai.provider';
import { DeepseekProvider } from './providers/deepseek.provider';
import { Category, CategorySchema } from '../category/schemas/category.schema';
import { GithubTrend, GithubTrendSchema } from '../github-trend/schemas/github-trend.schema';
import { IAiProvider } from './interfaces/ai-provider.interface';

@Module({
  imports: [
    MongooseModule.forFeature([
      { name: Category.name, schema: CategorySchema },
      { name: GithubTrend.name, schema: GithubTrendSchema },
    ]),
    ConfigModule,
  ],
  providers: [
    AiService,
    {
      provide: 'IAiProvider',
      useFactory: (configService: ConfigService) => {
        const aiProvider = configService.get('AI_PROVIDER');
        switch (aiProvider) {
          case 'openai':
            return new OpenAiProvider(configService);
          case 'deepseek':
            return new DeepseekProvider(configService);
          default:
            return new OpenAiProvider(configService);
        }
      },
      inject: [ConfigService],
    },
  ],
  exports: [AiService],
})
export class AiModule {} 