import { IsString, IsNumber } from 'class-validator';

export class GitHubTrendDto {
  @IsString()
  readonly name: string;

  @IsString()
  readonly url: string;

  @IsNumber()
  readonly stars: number;

  @IsString()
  readonly description: string;

  @IsString()
  readonly tags: string[];
} 