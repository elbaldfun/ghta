import { Schema } from 'mongoose';

export const GitHubTrendSchema = new Schema({
  name: { type: String, required: true },
  url: { type: String, required: true },
  stars: { type: Number, required: true },
  description: { type: String, required: true },
  tags: { type: [String], required: true },
}); 