import { Controller, Get, Query } from '@nestjs/common';
import { GithubTrendService } from './github-trend.service';
import { GithubTrend } from './schemas/github-trend.schema';
import { ApiOperation, ApiQuery, ApiResponse } from '@nestjs/swagger';

@Controller('trending')
export class GithubTrendController {
  constructor(private readonly githubTrendService: GithubTrendService) {}

  @Get()
  @ApiOperation({
    summary: '获取 GitHub 趋势仓库',
    description: '获取 GitHub 趋势仓库列表，支持多种过滤和排序选项'
  })
  @ApiQuery({
    name: 'minStars',
    required: false,
    type: Number,
    description: '最小 star 数量'
  })
  @ApiQuery({
    name: 'maxStars', 
    required: false,
    type: Number,
    description: '最大 star 数量'
  })
  @ApiQuery({
    name: 'language',
    required: false,
    type: String,
    description: '编程语言'
  })
  @ApiQuery({
    name: 'minIssues',
    required: false,
    type: Number,
    description: '最小 issue 数量'
  })
  @ApiQuery({
    name: 'maxIssues',
    required: false,
    type: Number,
    description: '最大 issue 数量'
  })
  @ApiQuery({
    name: 'limit',
    required: false,
    type: Number,
    description: '返回结果数量限制，默认 100'
  })
  @ApiQuery({
    name: 'sort',
    required: false,
    type: String,
    description: '排序方式，格式为 field:order，例如 starCount:desc'
  })
  @ApiResponse({
    status: 200,
    description: '成功获取趋势仓库列表',
    type: GithubTrend,
    isArray: true
  })
  async getTrendingRepos(
    @Query('minStars') minStars?: number,
    @Query('maxStars') maxStars?: number,
    @Query('language') language?: string,
    @Query('minIssues') minIssues?: number,
    @Query('maxIssues') maxIssues?: number,
    @Query('limit') limit?: number,
    @Query('sort') sort?: string,
  ): Promise<{data: GithubTrend[]}> {
    const filters: any = {};
    
    if (minStars) filters.minStars = Number(minStars);
    if (maxStars) filters.maxStars = Number(maxStars);
    if (language) filters.language = language;
    if (minIssues) filters.minIssues = Number(minIssues);
    if (maxIssues) filters.maxIssues = Number(maxIssues);
    if (limit) filters.limit = Number(limit);
    
    if (sort) {
      const [field, order] = sort.split(':');
      filters.sort = { [field]: order === 'desc' ? -1 : 1 };
    }

    const data = await this.githubTrendService.fetchTrendingRepos(filters);
    return {data};
  }
}
