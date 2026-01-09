export enum ModelClientType {
  OPENAI = 'openai',
  ANTHROPIC = 'anthropic',
  GOOGLE = 'google',
}

export interface BaseModel {
  /**
   * @description The unique identifier for the model
   * @example 'gpt-4o'
   */
  modelId: string

  /**
   * @description The base URL for the model
   * @example 'https://api.openai.com/v1'
   */
  baseUrl: string

  /**
   * @description The API key for the model
   * @example 'sk-1234567890'
   */
  apiKey: string

  /**
   * @description The client type for the model
   * @enum {ModelClientType}
   */
  clientType: ModelClientType

  /**
   * @description The display name for the model
   * @example 'GPT 4o'
   */
  name?: string
}