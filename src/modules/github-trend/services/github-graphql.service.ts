import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import axios from 'axios';
import { GithubTrend } from '../schemas/github-trend.schema';

@Injectable()
export class GithubGraphqlService {
  private readonly logger = new Logger(GithubGraphqlService.name);
  private readonly apiUrl = 'https://api.github.com/graphql';
  private requestCount = 0;
  private lastResetTime = Date.now();
  private readonly MAX_REQUESTS_PER_HOUR = 5000; // GitHub API 限制

  constructor(
    private configService: ConfigService,
    @InjectModel(GithubTrend.name) private GithubTrendSchema: Model<GithubTrend>,
  ) {}

  async fetchAllTrendingRepos(range: string) {
    let hasNextPage = true;
    let startCursor: string = ""; 
    let afterCursor: string = ""; 
  
    while (hasNextPage) {
      this.logger.debug(`fetchAllTrendingRepos, start to fetch, range is ${range}, startCursor is ${startCursor}, afterCursor is ${afterCursor}`)
      const response = await this.queryTrendingRepos(range, 95, afterCursor);
      
      if (response.data.search.pageInfo.startCursor === null) {
        this.logger.debug(`Github response no data for range ${range}, because of response.data.search.pageInfo.startCursor is null`)
        break;
      }
      // process response data
      await this.parseTrendingReposIntoDB(response);
      await new Promise(resolve => setTimeout(resolve, 300));
  
      // update cursor
      hasNextPage = response.data.search.pageInfo.hasNextPage;
      startCursor = response.data.search.pageInfo.startCursor;
      afterCursor = response.data.search.pageInfo.endCursor;
      
      if (hasNextPage) {
        this.logger.debug(`Current range is ${range}, hasNextPage is ${hasNextPage}, current cursor is ${startCursor}, the next cursor is ${afterCursor}`)
        await new Promise(resolve => setTimeout(resolve, 80));
        startCursor = afterCursor;
      }
    }
  }

  async queryTrendingRepos(range: string, first: number, after: string = ""): Promise<any> {
    try {

      // check and reset request count
      this.checkAndResetRequestCount();
      
      // check if exceed the hourly limit
      if (this.requestCount >= this.MAX_REQUESTS_PER_HOUR) {
        this.logger.error('Exceeded GitHub API hourly limit');
        throw new Error('Exceeded GitHub API hourly limit');
      }

      let search = after === "" 
        ? `query: "stars:${range}", type: REPOSITORY, first: ${first}`
        : `query: "stars:${range}", type: REPOSITORY, first: ${first}, after: "${after}"`;

      this.logger.debug(`GraphQL query parameters: ${search}`);
      
      const query = `
            query {
              search(${search}) {
                pageInfo {
                  hasPreviousPage
                  hasNextPage
                  startCursor
                  endCursor
                } 
                edges {
                  cursor
                  node {
                    ... on Repository {
                      name
                      owner {
                        login
                      }
                      repositoryTopics(first: 20) {
                        edges {
                          node {
                            topic {
                              name
                            }
                            url
                          }
                        }
                      }
                      description
                      stargazerCount
                      forkCount
                      pushedAt
                      primaryLanguage {
                        name
                      }
                      issues(states: OPEN) {
                        totalCount
                      }
                      releases(first: 5, orderBy: {field: CREATED_AT, direction: DESC}) {
                        edges {
                          node {
                            name
                            tagName
                            isPrerelease
                            isLatest
                            isDraft
                            publishedAt
                          }
                        }
                      }
                      url
                      homepageUrl
                      licenseInfo {
                        name
                        nickname
                        description
                      }
                    }
                  }
                }
              }
            }
          `

      const MAX_RETRIES = 5;
      let retryCount = 0;
      let lastError = null;

      while (retryCount < MAX_RETRIES) {
        try {
          const response = await axios.post(
            this.apiUrl,
            { query },
            {
              headers: {
                Authorization: `Bearer ${this.configService.get('GITHUB_API_TOKEN')}`,
              },
              timeout: 60000,
              validateStatus: (status) => status === 200,
            }
          );

          this.requestCount++;
          
          // check if there is any error in the response
          if (response.data.errors) {
            const errorMessage = response.data.errors.map(e => e.message).join(', ');
            throw new Error(`GraphQL error: ${errorMessage}`);
          }

          this.logger.debug(`Successfully fetched GitHub data: ${response.data.data.search?.edges[0]?.node?.url}`);
          return response.data;

        } catch (error) {
          lastError = error;
          
          // check if the error is due to rate limiting or server error
          const shouldRetry = error.response?.status === 429 || 
                            error.response?.status >= 500 ||
                            error.code === 'ECONNABORTED' ||
                            error.code === 'ECONNRESET';

          if (shouldRetry && retryCount < MAX_RETRIES - 1) {
            retryCount++;
            const delay = Math.pow(2, retryCount) * 1000;
            this.logger.warn(`Retry ${retryCount}/${MAX_RETRIES}, delay ${delay}ms, error: ${error.message}`);
            await new Promise(resolve => setTimeout(resolve, delay));
            continue;
          }

          // log the error
          this.logger.error(`Failed to fetch GitHub data`, {
            error: error.message,
            status: error.response?.status,
            statusText: error.response?.statusText,
            data: error.response?.data,
            code: error.code,
            retryCount
          });
          
          throw error;
        }
      }

      throw lastError;
      
    } catch (error) {
      this.logger.error(`Failed to fetch GitHub data: ${error.message}`);
      throw error;
    }
  }

  private checkAndResetRequestCount(): void {
    const now = Date.now();
    if (now - this.lastResetTime >= 30 * 1000) {
      this.requestCount = 0;
      this.lastResetTime = now;
      this.logger.debug(`Reset API request counter`);
    }
  }

  async parseTrendingReposIntoDB(response: any) {
    this.logger.debug(`Processing GitHub GraphQL response data, number of data: ${response.data.search.edges.length}`);
    
    for (const edge of response.data.search.edges) {
      const repo = edge.node;
      const repoNameID = repo.url.split('https://github.com/')[1];
      
      try {
        const existingRepo = await this.GithubTrendSchema.findOne({ repoNameID: repoNameID });
        
        if (existingRepo) {
          await this.GithubTrendSchema.updateOne(
            { repoNameID: repoNameID },
            {
              owner: repo.owner.login,
              name: repo.name,
              repoNameID: repoNameID,
              description: repo.description,
              starCount: repo.stargazerCount,
              forkCount: repo.forkCount,
              forkFromRepo: repo.forkFromRepository?.name,
              language: repo.primaryLanguage?.name,
              openIssuesCount: repo.issues.totalCount,
              latestRelease: repo.releases.edges[0]?.node || null,
              url: repo.url,
              homepageUrl: repo.homepageUrl,
              readme: repo.readme?.text,
              fetchedAt: new Date(),
            }
          );
          this.logger.log(`Update repository data successfully: ${repo.name}`);
        } else {
          await this.GithubTrendSchema.create({
            owner: repo.owner.login,
            name: repo.name,
            repoNameID: repoNameID,
            description: repo.description,
            starCount: repo.stargazerCount,
            forkCount: repo.forkCount,
            forkFromRepo: repo.forkFromRepository?.name,
            language: repo.primaryLanguage?.name,
            openIssuesCount: repo.issues.totalCount,
            latestRelease: repo.releases.edges[0]?.node || null,
            url: repo.url,
            homepageUrl: repo.homepageUrl,
            readme: repo.readme?.text,
            fetchedAt: new Date(),
          });
          this.logger.log(`Save new repository data successfully: ${repo.name}`);
        }
      } catch (error) {
        this.logger.error(`Failed to process repository data: ${repo.name}`, {
          error: error.message,
          repoNameID,
          url: repo.url
        });
      }
    }
  }
} 