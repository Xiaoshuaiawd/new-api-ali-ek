# Audio URL 功能实现说明

## 功能概述

为了支持 Gemini 2.0 Flash 模型的多模态音频处理能力，新增了 `audio_url` 字段支持。该功能与现有的 `image_url` 功能类似，允许通过 URL 传递音频文件进行处理。

## 修改内容

### 1. DTO 结构体修改 (dto/openai_request.go)

#### 新增结构体
```go
type MessageAudioUrl struct {
    Url      string `json:"url"`
    MimeType string
}

func (m *MessageAudioUrl) IsRemoteAudio() bool {
    return strings.HasPrefix(m.Url, "http")
}
```

#### MediaContent 结构体增加字段
```go
type MediaContent struct {
    Type       string `json:"type"`
    Text       string `json:"text,omitempty"`
    ImageUrl   any    `json:"image_url,omitempty"`
    AudioUrl   any    `json:"audio_url,omitempty"`  // 新增
    InputAudio any    `json:"input_audio,omitempty"`
    File       any    `json:"file,omitempty"`
    VideoUrl   any    `json:"video_url,omitempty"`
    CacheControl json.RawMessage `json:"cache_control,omitempty"`
}
```

#### 新增解析方法
```go
func (m *MediaContent) GetAudioMedia() *MessageAudioUrl {
    if m.AudioUrl != nil {
        if _, ok := m.AudioUrl.(*MessageAudioUrl); ok {
            return m.AudioUrl.(*MessageAudioUrl)
        }
        if itemMap, ok := m.AudioUrl.(map[string]any); ok {
            out := &MessageAudioUrl{
                Url:      common.Interface2String(itemMap["url"]),
                MimeType: common.Interface2String(itemMap["mime_type"]),
            }
            return out
        }
    }
    return nil
}
```

#### 新增常量
```go
const (
    ContentTypeText       = "text"
    ContentTypeImageURL   = "image_url"
    ContentTypeAudioURL   = "audio_url"  // 新增
    ContentTypeInputAudio = "input_audio"
    ContentTypeFile       = "file"
    ContentTypeVideoUrl   = "video_url"
)
```

### 2. Gemini 适配器修改 (relay/channel/gemini/relay-gemini.go)

在 CovertGemini2OpenAI 函数中添加了对 ContentTypeAudioURL 的处理逻辑，与 image_url 的处理方式完全一致。

## 使用示例

### 修改前的请求格式（使用 image_url）
```json
{
  "model": "gemini-2.0-flash",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "image_url": {
            "url": "https://download.samplelib.com/mp3/sample-3s.mp3"
          },
          "type": "image_url"
        },
        {
          "text": "please generate the audio description",
          "type": "text"
        }
      ]
    }
  ],
  "max_tokens": 4096
}
```

### 修改后的请求格式（使用 audio_url）
```json
{
  "model": "gemini-2.0-flash", 
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "audio_url": {
            "url": "https://download.samplelib.com/mp3/sample-3s.mp3"
          },
          "type": "audio_url"
        },
        {
          "text": "please generate the audio description",
          "type": "text"
        }
      ]
    }
  ],
  "max_tokens": 4096
}
```

## 技术要点

1. **向后兼容**: 现有的 image_url 和 input_audio 功能完全保持不变
2. **一致性**: audio_url 的处理逻辑与 image_url 保持一致
3. **类型安全**: 使用强类型结构体 MessageAudioUrl 确保类型安全
4. **错误处理**: 完整的错误处理链，包括 URL 下载失败、MIME 类型检查等
5. **白名单验证**: 音频文件的 MIME 类型会经过 Gemini 支持的白名单验证

## 支持的音频格式

根据 geminiSupportedMimeTypes 映射，支持的音频格式包括：
- audio/mpeg
- audio/mp3  
- audio/wav

## 注意事项

1. 该功能主要为 Gemini 2.0 Flash 等多模态模型设计
2. 音频 URL 必须是可公开访问的 HTTP/HTTPS 链接
3. 音频文件大小受到系统设置的最大下载限制
4. MIME 类型检查确保只有支持的音频格式能被处理 