export interface IAiProvider {
  analyze(prompt: string): Promise<string>;
} 