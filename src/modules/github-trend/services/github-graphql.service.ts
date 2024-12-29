import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import {inspect} from 'util'
import axios from 'axios';

@Injectable()
export class GithubGraphqlService {
  private readonly logger = new Logger(GithubGraphqlService.name);
  private readonly apiUrl = 'https://api.github.com/graphql';
  private requestCount = 0;
  private lastResetTime = Date.now();
  private readonly MAX_REQUESTS_PER_HOUR = 5000; // GitHub API 限制

  constructor(private configService: ConfigService) {}

  async queryTrendingRepos(): Promise<any> {
    try {
      // 检查并重置计数器
      this.checkAndResetRequestCount();
      
      // 检查是否超出限制
      if (this.requestCount >= this.MAX_REQUESTS_PER_HOUR) {
        throw new Error('Exceeded GitHub API hourly limit');
      }

      const response = await axios.post(
        this.apiUrl,
        {

                      //  homepageUrl
                      //  readme: object(expression: "HEAD:README.md") {
                      //   ... on Blob {
                      //     text
                      //   }
                      // }
          query: `
            query {
              search(query: "stars:>0", type: REPOSITORY, first: 1) {
                pageInfo {
                  hasPreviousPage
                  hasNextPage
                  startCursor
                  endCursor
                  totalCount
                } 
                edges {
                  cursor
                  node {
                    ... on Repository {
                      name
                      owner {
                        login
                      }
                      description
                      stargazerCount
                      forkCount
                      primaryLanguage {
                        name
                      }
                      issues(states: OPEN) {
                        totalCount
                      }
                      releases(first: 1, orderBy: {field: CREATED_AT, direction: DESC}) {
                        edges {
                          node {
                            name
                            tagName
                          }
                        }
                      }
                      url
                      homepageUrl
                    }
                  }
                }
              }
            }
          `
        },
        {
          headers: {
            Authorization: `Bearer ${this.configService.get('GITHUB_API_TOKEN')}`,
          },
        }
      );

      this.requestCount++;

      
      this.logger.debug(`Success to fetch GitHub data: ${response.data.data.search.edges[0].node.url}`);
      this.logger.debug(`graphql response data: ${inspect(response, {showHidden: true, depth: null})}`)
      return response.data;
    } catch (error) {
      this.logger.error(`Failed to fetch GitHub data: ${error.message}`);
      throw error;
    }
  }

  private checkAndResetRequestCount(): void {
    const now = Date.now();
    if (now - this.lastResetTime >= 30 * 1000) { // interval 30s 
      this.requestCount = 0;
      this.lastResetTime = now;
    }
  }
} 