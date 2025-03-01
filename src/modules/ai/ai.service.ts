import { Injectable, Logger } from '@nestjs/common';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { ConfigService } from '@nestjs/config';
import { OpenAI } from 'openai';
import { Category } from '../category/schemas/category.schema';
import { GitHubTrend } from '../github-trend/schemas/github-trend.schema';
import { ICategoryAnalysisResult } from './interfaces/ai.interface';

@Injectable()
export class AiService {
  private readonly logger = new Logger(AiService.name);
  private readonly openai: OpenAI;

  constructor(
    @InjectModel(Category.name) private categoryModel: Model<Category>,
    @InjectModel(GitHubTrend.name) private githubTrendModel: Model<GitHubTrend>,
    private configService: ConfigService,
  ) {
    this.openai = new OpenAI({
      apiKey: this.configService.get<string>('OPENAI_API_KEY'),
    });
  }

  async analyzeRepository(): Promise<ICategoryAnalysisResult> {
    try {
      // 1. 获取所有分类
      const categories = await this.categoryModel.find().exec();
      
      // 2. 获取一个 GitHub 仓库记录
      const repo = await this.githubTrendModel.findOne().exec();
      if (!repo) {
        throw new Error('No GitHub repository found');
      }

      // 3. 构建 AI 提示
      const prompt = this.buildAIPrompt(categories, repo);

      // 4. 调用 OpenAI API 进行分析
      const completion = await this.openai.chat.completions.create({
        model: "gpt-3.5-turbo",
        messages: [
          {
            role: "system",
            content: "You are a technical expert who categorizes GitHub repositories based on their content and purpose."
          },
          {
            role: "user",
            content: prompt
          }
        ],
        temperature: 0.2,
      });

      // 5. 解析 AI 响应
      const result = this.parseAIResponse(completion.choices[0].message.content);

      // 6. 如果需要创建新分类
      if (result.isNewCategory) {
        const newCategory = await this.createNewCategory(result, repo);
        return {
          categoryId: newCategory._id.toString(),
          path: newCategory.path,
          isNewCategory: true
        };
      }

      return result;
    } catch (error) {
      this.logger.error(`Error analyzing repository: ${error.message}`);
      throw error;
    }
  }

  private buildAIPrompt(categories: Category[], repo: GitHubTrend): string {
    const categoryTree = this.buildCategoryTree(categories);
    return `
Please analyze this GitHub repository and categorize it based on the existing category structure:

Repository Information:
- Name: ${repo.name}
- Description: ${repo.description}
- Language: ${repo.language}
- Topics: ${repo.topics.join(', ')}

Existing Categories:
${categoryTree}

Please respond in JSON format with the following structure:
{
  "categoryId": "existing-category-id or NEW",
  "path": "existing-path or suggested-new-path",
  "isNewCategory": boolean,
  "suggestedName": "only if isNewCategory is true"
}
`;
  }

  private buildCategoryTree(categories: Category[]): string {
    // 构建分类树的字符串表示
    const rootCategories = categories.filter(c => !c.parentId);
    return rootCategories.map(cat => this.buildCategoryBranch(cat, categories, 0)).join('\n');
  }

  private buildCategoryBranch(category: Category, allCategories: Category[], level: number): string {
    const indent = '  '.repeat(level);
    const children = allCategories.filter(c => c.parentId.toString() === category._id.toString());
    const childrenStr = children.map(child => 
      this.buildCategoryBranch(child, allCategories, level + 1)
    ).join('\n');
    
    return `${indent}- ${category.name} (ID: ${category._id}, Path: ${category.path})${childrenStr ? '\n' + childrenStr : ''}`;
  }

  private parseAIResponse(response: string): ICategoryAnalysisResult {
    try {
      const parsed = JSON.parse(response);
      return {
        categoryId: parsed.categoryId,
        path: parsed.path,
        isNewCategory: parsed.isNewCategory,
      };
    } catch (error) {
      throw new Error('Failed to parse AI response');
    }
  }

  private async createNewCategory(result: ICategoryAnalysisResult, repo: GitHubTrend): Promise<Category> {
    const pathParts = result.path.split('/').filter(Boolean);
    const level = pathParts.length;
    const parentPath = pathParts.slice(0, -1).join('/');
    
    const parent = parentPath ? 
      await this.categoryModel.findOne({ path: parentPath }) :
      null;

    const newCategory = await this.categoryModel.create({
      name: pathParts[pathParts.length - 1],
      path: result.path,
      parentId: parent?._id,
      level,
    });

    return newCategory;
  }
}
