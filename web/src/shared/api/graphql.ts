type GraphQLResponse<T> = { data?: T; errors?: Array<{ message: string }> }

async function readGraphQLFailure(response: Response, fallback: string): Promise<Error> {
  let detail = ''
  try {
    const body = await response.json() as GraphQLResponse<unknown>
    detail = body.errors?.[0]?.message || ''
  } catch {
    // Keep the HTTP status when the gateway did not return GraphQL JSON.
  }
  return new Error(detail || `${fallback} (HTTP ${response.status})`)
}

export async function graphql<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  const endpoint = import.meta.env.VITE_GRAPHQL_URL || '/graphql'
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ query, variables }),
  })
  if (!response.ok) throw await readGraphQLFailure(response, 'Dịch vụ GoshopX hiện chưa phản hồi. Hãy thử lại.')
  const body = await response.json() as GraphQLResponse<T>
  if (body.errors?.length) throw new Error(body.errors[0].message)
  if (!body.data) throw new Error('Dữ liệu GraphQL phản hồi không hợp lệ.')
  return body.data
}

export async function graphqlUpload<T>(query: string, variables: Record<string, unknown>, file: File, variablePath = 'variables.file'): Promise<T> {
  const endpoint = import.meta.env.VITE_GRAPHQL_URL || '/graphql'
  const body = new FormData()
  body.append('operations', JSON.stringify({ query, variables }))
  body.append('map', JSON.stringify({ '0': [variablePath] }))
  body.append('0', file)
  const response = await fetch(endpoint, { method: 'POST', credentials: 'include', body })
  if (!response.ok) throw await readGraphQLFailure(response, 'Không thể tải hình ảnh lên máy chủ.')
  const result = await response.json() as GraphQLResponse<T>
  if (result.errors?.length) throw new Error(result.errors[0].message)
  if (!result.data) throw new Error('Dữ liệu GraphQL phản hồi không hợp lệ.')
  return result.data
}
