import { Injectable, Logger, Inject } from '@nestjs/common';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { ConfigService } from '@nestjs/config';
import { Category } from '../category/schemas/category.schema';
import { GithubTrend } from '../github-trend/schemas/github-trend.schema';
import { ICategoryAnalysisResult } from './interfaces/ai.interface';
import { IAiProvider } from './interfaces/ai-provider.interface';
import { Cron, CronExpression } from '@nestjs/schedule';
@Injectable()
export class AiService {
  private readonly logger = new Logger(AiService.name);

  constructor(
    @InjectModel(Category.name) private categoryModel: Model<Category>,
    @InjectModel(GithubTrend.name) private githubTrendModel: Model<GithubTrend>,
    private configService: ConfigService,
    @Inject('IAiProvider') private aiProvider: IAiProvider,
  ) {}

  // @Cron(CronExpression.EVERY_6_HOURS)
  @Cron('0 10 08 * * *') // 每天下午14:50运行
  async analyzeRepositoriesTask() {
    try {
      this.logger.log('Starting repository analysis task...');

      // Get all categories
      const categories = await this.categoryModel.find().exec();

      // Get unanalyzed repositories
      const unanalyzedRepos = await this.githubTrendModel
        .find({ categoryId: { $size: 0 } })
        .limit(1)
        .exec();

      this.logger.log(`Found ${unanalyzedRepos.length} repositories to analyze`);

      // Analyze each repository
      for (const repo of unanalyzedRepos) {
        try {
          const result = await this.analyzeRepository(categories, repo);
          
          // Update repository with category info
          await this.githubTrendModel.findByIdAndUpdate(repo._id, {
            categoryId: result.categoryId,
            categoryPath: result.path
          });

          this.logger.debug(`Successfully analyzed repository: ${repo.name}`);
        } catch (error) {
          this.logger.error(`Failed to analyze repository ${repo.name}: ${error.message}`);
          continue;
        }
      }

      this.logger.log('Completed repository analysis task');
    } catch (error) {
      this.logger.error(`Error in repository analysis task: ${error.message}`);
    }
  }


  async analyzeRepository(categories: Category[], repo: GithubTrend): Promise<ICategoryAnalysisResult> {
    try {
      const prompt = this.buildAIPrompt(categories, repo);

      const response = await this.aiProvider.analyze(prompt);

      const result = this.parseAIResponse(response);

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

  private buildAIPrompt(categories: Category[], repo: GithubTrend): string {
    const categoryTree = this.buildCategoryTree(categories);
    return `
Please analyze this GitHub repository and categorize it based on the existing category structure:

Repository Information:
- Name: ${repo.name}
- Description: ${repo.description}
- Language: ${repo.language}
- Topics: ${repo.repoTopics.join(', ')}

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
        newPath: parsed.newPath,
      };
    } catch (error) {
      throw new Error('Failed to parse AI response');
    }
  }

  private async createNewCategory(result: ICategoryAnalysisResult, repo: GithubTrend): Promise<Category> {
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
