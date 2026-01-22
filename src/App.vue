<!-- src/App.vue -->
<template>
  <div class="container">
    <div class="app-header">
      <h1>跨端剪贴板共享</h1>
      <div v-if="isAuthenticated" class="device-badge">
        <span class="device-icon" :class="deviceIconClass"></span>
        {{ deviceInfo }}
      </div>
    </div>

    <!-- 登录部分 -->
    <form v-if="!isAuthenticated" class="auth-container" @submit.prevent="authenticate">
      <h2>请登录</h2>
      <div class="form-group">
        <label for="device">设备标识</label>
        <input
            id="device"
            v-model="deviceInfo"
            type="text"
            placeholder="设备标识将自动检测"
            readonly
        />
        <small class="form-hint">设备类型已自动识别</small>
      </div>
      <div class="form-group">
        <label for="password">密码</label>
        <input
            id="password"
            v-model="password"
            type="password"
            placeholder="请输入密码"
        />
      </div>
      <button class="primary-button" type="submit" :disabled="isAuthenticating">
        <span class="button-icon">🔐</span>
        {{ isAuthenticating ? '登录中...' : '登录' }}
      </button>
      <p v-if="authError" class="error">{{ authError }}</p>
    </form>

    <!-- 主界面 -->
    <div v-else class="main-container">
      <div class="clipboard-input">
        <h2>添加到剪贴板</h2>
        <div class="tabs">
          <button
              class="tab-button"
              :class="{ active: activeTab === 'text' }"
              @click="activeTab = 'text'"
          >
            <span class="tab-icon">📝</span> 文本
          </button>
          <button
              class="tab-button"
              :class="{ active: activeTab === 'image' }"
              @click="activeTab = 'image'"
          >
            <span class="tab-icon">🖼️</span> 图片
          </button>
        </div>

        <div v-if="activeTab === 'text'" class="tab-content">
          <textarea
              v-model="newClipboardContent"
              placeholder="粘贴文本内容到这里"
              @focus="tryReadClipboard"
          ></textarea>
          <div class="action-buttons">
            <button class="secondary-button" @click="newClipboardContent = ''">
              清空
            </button>
            <button
                class="primary-button"
                @click="addToClipboard"
                :disabled="!newClipboardContent || isUploading"
            >
              <span class="button-icon">{{ isUploading ? '⏳' : '➕' }}</span>
              {{
                isUploading
                    ? (uploadProgress !== null ? `上传中... ${uploadProgress}%` : '上传中...')
                    : '添加到共享剪贴板'
              }}
            </button>
          </div>
        </div>

        <div v-else-if="activeTab === 'image'" class="tab-content">
          <div class="image-preview-area">
            <div
                v-if="selectedImage"
                class="image-preview"
            >
              <img :src="imagePreviewUrl" alt="Image preview"/>
              <button class="remove-image" @click="clearImageSelection">✖</button>
            </div>
            <div v-else class="image-dropzone" @drop.prevent="handleImageDrop" @dragover.prevent>
              <input
                  id="image-input"
                  type="file"
                  accept="image/*"
                  @change="handleImageSelect"
                  class="hidden-input"
              />
              <label for="image-input" class="dropzone-label">
                <span class="upload-icon">📷</span>
                <span>拖放图片到这里、点击选择、或按 Ctrl+V 粘贴</span>
              </label>
            </div>
          </div>
          <div class="action-buttons">
            <button class="secondary-button" @click="clearImageSelection" v-if="selectedImage">
              清除
            </button>
            <button
                class="primary-button"
                @click="addToClipboard"
                :disabled="!selectedImage || isUploading"
            >
              <span class="button-icon">{{ isUploading ? '⏳' : '➕' }}</span>
              {{
                isUploading
                    ? (uploadProgress !== null ? `上传中... ${uploadProgress}%` : '上传中...')
                    : '添加到共享剪贴板'
              }}
            </button>
          </div>
        </div>
      </div>

      <div class="clipboard-history">
        <h2>剪贴板历史记录</h2>
        <div class="history-controls">
          <div class="search-control">
            <input
                v-model="searchQuery"
                type="text"
                placeholder="搜索剪贴板内容或设备"
                @keyup.enter="searchClipboardHistory"
            />
            <button
                class="secondary-button"
                @click="searchClipboardHistory"
                :disabled="searchLoading || !searchQuery.trim()"
            >
              搜索
            </button>
            <button
                v-if="searchQuery"
                class="ghost-button"
                @click="clearSearch"
                :disabled="searchLoading"
            >
              清除
            </button>
          </div>
          <div class="refresh-control">
            <span>自动刷新: </span>
            <label class="switch">
              <input type="checkbox" v-model="autoRefresh">
              <span class="slider round"></span>
            </label>
            <button class="refresh-button" @click="reload">
              <span class="refresh-icon">🔄</span>
            </button>
          </div>
        </div>
        <p v-if="searchError" class="error search-error">{{ searchError }}</p>

        <div v-if="listLoading" class="loading-container">
          <div class="spinner"></div>
          <span>{{ isSearchMode ? '搜索中...' : '加载中...' }}</span>
        </div>

        <div v-else-if="displayItems.length === 0" class="empty-state">
          <span class="empty-icon">📋</span>
          <p>{{ isSearchMode ? '没有匹配的剪贴板记录' : '暂无剪贴板记录' }}</p>
          <p v-if="!isSearchMode" class="empty-hint">添加内容后将显示在这里</p>
        </div>

        <div v-else class="clipboard-list" :key="listKey">
          <div
              v-for="(item, index) in displayItems"
              :key="item.id"
              class="clipboard-item"
              :class="{ 'highlighter': index < 1 }"
          >
            <div class="item-header">
              <div class="device-info">
                <span class="device-icon" :class="getDeviceIconClass(item.deviceInfo)"></span>
                {{ item.deviceInfo }}
              </div>
              <span class="timestamp" :title="formatFullTime(item.createdAt)">
                {{ formatTime(item.createdAt) }}
              </span>
            </div>

            <div class="item-content">
              <!-- 文本内容 -->
              <div v-if="item.type === 'text'" class="text-content">
                {{ item.content }}
              </div>

              <!-- 图片内容 -->
              <div v-else-if="item.type === 'image'" class="image-content">
                <img :src="`data:image/png;base64,${item.imageData}`" alt="Clipboard image"
                     @click="previewImage(item)"/>
              </div>
            </div>

            <!-- 前三项的快速操作按钮 -->
            <div v-if="item.type === 'text'" class="item-actions">
              <button class="action-button" @click="copyToClipboard(item.content)" title="复制全部">
                <span class="action-icon">📋</span> 复制
              </button>
              <button class="action-button" @click="splitAndShowWords(item.content, index)" title="智能拆分文本">
                <span class="action-icon">✂️</span> 拆词
              </button>

              <!-- 拆词结果 -->
              <div v-if="wordSplitResults[index] && wordSplitResults[index].length > 0" class="split-words">
                <div
                    v-for="(word, wordIndex) in wordSplitResults[index]"
                    :key="wordIndex"
                    class="word-chip"
                    :class="{ selected: selectedWords[index]?.some(selected => selected.wordIndex === wordIndex) }"
                    @click="toggleWordSelection(word, index, wordIndex)"
                    :title="`点击选择: ${word}`"
                >
                  {{ word }}
                </div>
                <button class="action-button" @click="copyMergedWords(index)" title="复制合并的分词">
                  <span class="action-icon">📋</span> 复制合并
                </button>
              </div>
            </div>

            <div v-if="item.type === 'image'" class="item-actions">
              <button class="action-button" @click="downloadImage(item.imageData)" title="下载图片">
                <span class="action-icon">💾</span> 下载
              </button>
              <button class="action-button" @click="previewImage(item)" title="预览图片">
                <span class="action-icon">🔍</span> 预览
              </button>
            </div>
          </div>
          <div v-if="!isSearchMode && loadingMore" class="load-more-spinner">
            <div class="spinner"></div>
            <span>加载更多...</span>
          </div>
          <div v-if="!isSearchMode && noMoreData && clipboardItems.length > 0" class="no-more-data">
            没有更多数据了
          </div>
        </div>
      </div>
    </div>

    <!-- 图片预览模态框 -->
    <div v-if="imagePreviewModal" class="modal-overlay" @click="imagePreviewModal = false">
      <div class="modal-content" @click.stop>
        <button class="modal-close" @click="imagePreviewModal = false">✖</button>
        <img :src="`data:image/png;base64,${currentPreviewImage}`" alt="Preview" class="preview-image"/>
        <div class="modal-actions">
          <button class="action-button" @click="downloadImage(currentPreviewImage)">
            <span class="action-icon">💾</span> 下载图片
          </button>
        </div>
      </div>
    </div>

    <div v-if="notification.visible" class="global-notification" :class="notification.type">
      {{ notification.message }}
    </div>
  </div>
  <button class="scroll-to-top-button" @click="scrollToTop"></button>


</template>

<script lang="ts">
import {defineComponent, ref, onMounted, computed, watch, onUnmounted, type UnwrapRef} from 'vue';
import axios from 'axios';

const API_URL = `${import.meta.env.VITE_API_URL}${import.meta.env.VITE_APP_API_PORT ? `:${import.meta.env.VITE_APP_API_PORT}` : ''}`;


interface ClipboardItem {
  id: number;
  content: string;
  deviceInfo: string;
  type: 'text' | 'image';
  imageData?: string;
  createdAt: string;
}

interface NegClipboardItem {
  content: string;
  deviceInfo: string;
  type: 'text' | 'image';
  imageData?: string;
}

export default defineComponent({
  name: 'App',
  setup() {
    // 身份认证相关
    const isAuthenticated = ref(false);
    const isAuthenticating = ref(false);
    const authError = ref('');
    const deviceInfo = ref('');
    const password = ref('');
    const token = ref('');

    // 剪贴板相关
    const clipboardItems = ref<ClipboardItem[]>([]);
    const searchQuery = ref('');
    const searchResults = ref<ClipboardItem[]>([]);
    const searchLoading = ref(false);
    const searchError = ref('');
    const newClipboardContent = ref('');
    const selectedImage = ref<File | null>(null);
    const isUploading = ref(false);
    const uploadProgress = ref<number | null>(null);
    const loading = ref(false);
    const wordSplitResults = ref<Record<number, string[]>>({});
    const activeTab = ref('text');
    const autoRefresh = ref(true);
    let refreshInterval: number | null = null;
    const lastSharedContent = ref<ClipboardItem | null>(null);
    const lastSyncedContent = ref<NegClipboardItem>({
      content: '',
      deviceInfo: '',
      type: 'text',
    });
    const pollingInterval = ref<number | null>(null);

    const oldestItemId = ref<number | null>(null);
    const newestItemId = ref<number | null>(null);
    const loadingMore = ref(false);
    const noMoreData = ref(false);
    const isSearchMode = computed(() => searchQuery.value.trim().length > 0);
    const displayItems = computed(() => (isSearchMode.value ? searchResults.value : clipboardItems.value));
    const listLoading = computed(() => (isSearchMode.value ? searchLoading.value : loading.value));
    const listKey = computed(() => (isSearchMode.value ? `search-${searchQuery.value}` : newestItemId.value?.toString() || 'defaultKey'));

    const dedupeItemsById = (items: ClipboardItem[]) => {
      const seen = new Set<number>();
      const result: ClipboardItem[] = [];
      for (const item of items) {
        if (seen.has(item.id)) continue;
        seen.add(item.id);
        result.push(item);
      }
      return result;
    };

    const mergeIncomingToTop = (incoming: ClipboardItem[], current: ClipboardItem[]) => {
      // Keep incoming order first, then existing; drop duplicates by id.
      return dedupeItemsById([...incoming, ...current]);
    };

    const mergeIncomingToBottom = (incoming: ClipboardItem[], current: ClipboardItem[]) => {
      // Keep existing order first, then incoming (used for load-more); drop duplicates by id.
      return dedupeItemsById([...current, ...incoming]);
    };

    const normalizeTextForHash = (text: string) => {
      // Keep server/client hash stable across OS clipboard newline differences.
      return text.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    };

    const recomputeBounds = (items: ClipboardItem[]) => {
      if (items.length === 0) {
        oldestItemId.value = 0;
        newestItemId.value = 0;
        lastSharedContent.value = null;
        return;
      }

      let minId = items[0].id;
      let maxId = items[0].id;
      let latest = items[0];

      for (const item of items) {
        if (item.id < minId) minId = item.id;
        if (item.id > maxId) {
          maxId = item.id;
          latest = item;
        }
      }

      oldestItemId.value = minId;
      newestItemId.value = maxId;
      lastSharedContent.value = latest;
    };

    // 图片预览
    const imagePreviewModal = ref(false);
    const currentPreviewImage = ref('');
    const imagePreviewUrl = ref('');
    const selectedWords = ref<Record<number, { word: string, wordIndex: number }[]>>({});


    // 设备类型检测
    const deviceIconClass = computed(() => {
      return getDeviceIconClass(deviceInfo.value);
    });

    // 检测设备类型
    const detectDeviceType = () => {
      const userAgent = navigator.userAgent;
      let deviceType = 'Unknown';

      // 检测常见设备类型
      if (/iPhone|iPad|iPod/i.test(userAgent)) {
        deviceType = userAgent.match(/iPhone/) ? 'iPhone' : 'iPad';
      } else if (/Android/i.test(userAgent)) {
        deviceType = 'Android';
        if (/Mobile/i.test(userAgent)) {
          deviceType = 'Android手机';
        } else {
          deviceType = 'Android平板';
        }
      } else if (/Windows/i.test(userAgent)) {
        deviceType = 'Windows电脑';
        if (/Windows Phone/i.test(userAgent)) {
          deviceType = 'Windows手机';
        }
      } else if (/Macintosh/i.test(userAgent)) {
        deviceType = 'Mac电脑';
      } else if (/Linux/i.test(userAgent)) {
        deviceType = 'Linux电脑';
      }

      // 添加浏览器信息
      if (/Chrome/i.test(userAgent) && !/Chromium|Edge/i.test(userAgent)) {
        deviceType += ' Chrome';
      } else if (/Firefox/i.test(userAgent)) {
        deviceType += ' Firefox';
      } else if (/Safari/i.test(userAgent) && !/Chrome|Chromium|Edge/i.test(userAgent)) {
        deviceType += ' Safari';
      } else if (/Edge/i.test(userAgent)) {
        deviceType += ' Edge';
      }

      return deviceType;
    };

    const getDeviceIconClass = (device: string) => {
      if (/iPhone|iPad|iPod/i.test(device)) {
        return 'device-ios';
      } else if (/Android/i.test(device)) {
        return 'device-android';
      } else if (/Windows/i.test(device)) {
        return 'device-windows';
      } else if (/Mac/i.test(device)) {
        return 'device-mac';
      } else if (/Linux/i.test(device)) {
        return 'device-linux';
      }
      return 'device-unknown';
    };

    // 检查是否已经认证
    onMounted(async () => {
      // 设置自动检测的设备名称
      deviceInfo.value = detectDeviceType();
      lastSyncedContent.value.deviceInfo = deviceInfo.value;

      setupScrollListener();
      document.addEventListener('paste', handlePaste);
      document.addEventListener('visibilitychange', handleVisibilityChange);

      const savedToken = localStorage.getItem('clipboard_token');
      const savedDevice = localStorage.getItem('clipboard_device');
      const tokenExpiry = localStorage.getItem('clipboard_token_expiry');

      if (savedToken && savedDevice && tokenExpiry) {
        // 检查token是否过期
        const expiryTime = parseInt(tokenExpiry);
        if (expiryTime > Date.now()) {
          token.value = savedToken;
          deviceInfo.value = savedDevice;
          isAuthenticated.value = true;
          await fetchClipboardItems();

          // 尝试获取系统剪贴板（仅在支持的浏览器中）
          await tryReadClipboard();
          startPolling();
        } else {
          // Token已过期，清除本地存储
          localStorage.removeItem('clipboard_token');
          localStorage.removeItem('clipboard_device');
          localStorage.removeItem('clipboard_token_expiry');
        }
      }

    });

    // 监听自动刷新开关
    watch(autoRefresh, (newValue) => {
      if (newValue) {
        startPolling();
      } else {
        stopPolling();
      }
    });

    watch(searchQuery, (newValue) => {
      if (!newValue.trim()) {
        searchResults.value = [];
        searchError.value = '';
      }
    });

    // 设置刷新间隔
    // const setupRefreshInterval = () => {
    //   if (autoRefresh.value && refreshInterval === null) {
    //     refreshInterval = window.setInterval(() => {
    //       if (isAuthenticated.value) {
    //         fetchClipboardItems(); // 只刷新剪贴板历史记录
    //       }
    //     }, 1000);
    //   }
    // };

    let pollingStopRequested = false;
    let pollingInFlight = false;

    const pollingTick = async () => {
      if (pollingInFlight) return;
      pollingInFlight = true;
      try {
        if (isAuthenticated.value) {
          await fetchLastSharedContent();
          await checkClipboard();
        }
      } catch (error) {
        console.error('Polling tick failed:', error);
      } finally {
        pollingInFlight = false;
      }

      if (pollingStopRequested || !autoRefresh.value || !isAuthenticated.value) {
        stopPolling();
        return;
      }

      pollingInterval.value = window.setTimeout(() => {
        pollingInterval.value = null;
        void pollingTick();
      }, 1500);
    };

    // 启动轮询（非重叠，网络卡顿时也不会堆积多个并发请求）
    const startPolling = () => {
      if (!autoRefresh.value) return;
      pollingStopRequested = false;
      if (pollingInterval.value !== null) return;
      void pollingTick();
    };

    // 停止轮询
    const stopPolling = () => {
      pollingStopRequested = true;
      if (pollingInterval.value !== null) {
        clearTimeout(pollingInterval.value);
        pollingInterval.value = null;
      }
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        if (isAuthenticated.value) {
          void fetchLastSharedContent();
        }
        if (autoRefresh.value) {
          startPolling();
        }
      } else {
        stopPolling();
      }
    };

    let scrollListenerAttached = false;
    const handleScroll = () => {
      if (isSearchMode.value || noMoreData.value || loadingMore.value) return;

      const scrollPosition = window.innerHeight + window.pageYOffset;
      const documentHeight = document.documentElement.offsetHeight;

      // 当滚动到距离底部100px时，加载更多
      if (documentHeight - scrollPosition < 100) {
        fetchClipboardItems(true);
      }
    };

    const setupScrollListener = () => {
      if (scrollListenerAttached) return;
      scrollListenerAttached = true;
      window.addEventListener('scroll', handleScroll);
      onUnmounted(() => {
        window.removeEventListener('scroll', handleScroll);
      });
    };

    // 组件销毁时清除定时器
    onUnmounted(() => {
      if (refreshInterval !== null) {
        clearInterval(refreshInterval);
      }
      stopPolling();
      document.removeEventListener('paste', handlePaste);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    });

    const reload = () => {
      window.location.reload();
      return;
    };

    // 认证
    const authenticate = async () => {
      if (!deviceInfo.value || !password.value) {
        authError.value = '设备标识和密码不能为空';
        return;
      }

      isAuthenticating.value = true;
      authError.value = '';

      try {
        const response = await axios.post(`${API_URL}/auth`, {
          password: password.value,
          deviceInfo: deviceInfo.value
        });

        token.value = response.data.token;
        isAuthenticated.value = true;

        // 保存到本地存储，设置过期时间为24小时
        const expiryTime = Date.now() + 24 * 60 * 60 * 1000;
        localStorage.setItem('clipboard_token', token.value);
        localStorage.setItem('clipboard_device', deviceInfo.value);
        localStorage.setItem('clipboard_token_expiry', expiryTime.toString());

        // 获取剪贴板项目
        await fetchClipboardItems();

        // 尝试获取系统剪贴板（仅在支持的浏览器中）
        await tryReadClipboard();
        startPolling();
      } catch (error) {
        authError.value = '认证失败，请检查密码是否正确';
        console.error('Authentication error:', error);
      } finally {
        isAuthenticating.value = false;
      }
    };

    // 尝试读取系统剪贴板
    const tryReadClipboard = async () => {
      if (!navigator.clipboard || !navigator.clipboard.readText) {
        console.log('Clipboard API not supported or permission not granted');
        return;
      }

      try {
        const text = await navigator.clipboard.readText();
        if (text && text.trim() !== '') {
          newClipboardContent.value = text;
          lastSyncedContent.value.content = text;
        }
      } catch (error) {
        console.log('Cannot read clipboard, may need permission:', error);
      }
    };

    // 获取剪贴板项目
    const fetchClipboardItems = async (loadMore = false) => {
      if (!isAuthenticated.value) return;

      if (loadMore && loadingMore.value) return; // 防止重复加载

      if (loadMore) {
        loadingMore.value = true;
      } else if (!loadMore) {
        loading.value = true;
      }

      try {
        let url = `${API_URL}/api/clipboard`;

        // 如果是加载更多，传递oldestItemId作为参数
        if (loadMore && oldestItemId.value) {
          url += `?old=${oldestItemId.value}`;
        }

        const response = await axios.get(url, {
          headers: {
            Authorization: `Bearer ${token.value}`,
          },
        });

        const incomingItems: ClipboardItem[] = response.data || [];

        if (incomingItems.length > 0) {
          if (loadMore) {
            clipboardItems.value = mergeIncomingToBottom(incomingItems, clipboardItems.value);
          } else {
            noMoreData.value = false;
            clipboardItems.value = dedupeItemsById(incomingItems);
          }
          recomputeBounds(clipboardItems.value);
        } else if (loadMore) {
          noMoreData.value = true;
        } else {
          clipboardItems.value = [];
          noMoreData.value = false;
          recomputeBounds(clipboardItems.value);
        }
      } catch (error) {
        console.error('Error fetching clipboard items:', error);

        // 如果 token 过期，处理认证状态
        if (axios.isAxiosError(error) && error.response?.status === 401) {
          isAuthenticated.value = false;
          localStorage.removeItem('clipboard_token');
          localStorage.removeItem('clipboard_device');
          localStorage.removeItem('clipboard_token_expiry');
          stopPolling();
        }
      } finally {
        if (loadMore) {
          loadingMore.value = false;
        } else {
          loading.value = false;
        }
      }
    };

    const searchClipboardHistory = async () => {
      if (!isAuthenticated.value) return;

      const query = searchQuery.value.trim();
      if (!query) {
        searchResults.value = [];
        searchError.value = '';
        return;
      }

      searchLoading.value = true;
      searchError.value = '';

      try {
        const response = await axios.get(`${API_URL}/api/clipboard/search`, {
          params: {
            q: query,
            limit: 100,
          },
          headers: {
            Authorization: `Bearer ${token.value}`,
          },
        });
        searchResults.value = response.data || [];
      } catch (error) {
        console.error('Error searching clipboard items:', error);
        searchError.value = '搜索失败，请稍后重试';
        if (axios.isAxiosError(error) && error.response?.status === 401) {
          isAuthenticated.value = false;
          localStorage.removeItem('clipboard_token');
          localStorage.removeItem('clipboard_device');
          localStorage.removeItem('clipboard_token_expiry');
          stopPolling();
        }
      } finally {
        searchLoading.value = false;
      }
    };

    const clearSearch = () => {
      searchQuery.value = '';
      searchResults.value = [];
      searchError.value = '';
    };


    // 添加到剪贴板
    const addToClipboard = async () => {
      if (!isAuthenticated.value) return;
      if (isUploading.value) return;

      isUploading.value = true;
      uploadProgress.value = null;
      try {
        let payload = {};

        if (activeTab.value === 'image' && selectedImage.value) {
          // 处理图片
          const reader = new FileReader();
          const imagePromise = new Promise((resolve) => {
            reader.onload = (e) => {
              const base64 = (e.target?.result as string).split(',')[1];
              resolve(base64);
            };
          });

          reader.readAsDataURL(selectedImage.value);
          const base64Image = await imagePromise;

          payload = {
            content: selectedImage.value.name,
            deviceInfo: deviceInfo.value,
            type: 'image',
            imageData: base64Image
          };
        } else if (activeTab.value === 'text' && newClipboardContent.value) {
          // 处理文本
          payload = {
            content: newClipboardContent.value,
            deviceInfo: deviceInfo.value,
            type: 'text'
          };
        } else {
          return;
        }

        const response = await axios.post(`${API_URL}/api/clipboard`, payload, {
          headers: {
            'Authorization': `Bearer ${token.value}`
          },
          onUploadProgress: (progressEvent) => {
            const total = progressEvent.total;
            if (!total) return;
            uploadProgress.value = Math.round((progressEvent.loaded / total) * 100);
          },
        });

        const createdItem = response.data as ClipboardItem;
        clipboardItems.value = mergeIncomingToTop([createdItem], clipboardItems.value);
        recomputeBounds(clipboardItems.value);

        // 重置表单
        if (activeTab.value === 'text') {
          newClipboardContent.value = '';
        } else {
          clearImageSelection();
        }
      } catch (error) {
        console.error('Error adding to clipboard:', error);
        if (axios.isAxiosError(error) && error.response?.status === 401) {
          isAuthenticated.value = false;
          localStorage.removeItem('clipboard_token');
          localStorage.removeItem('clipboard_device');
          localStorage.removeItem('clipboard_token_expiry');
          stopPolling();
        }
      } finally {
        isUploading.value = false;
        uploadProgress.value = null;
      }
    };

    const blobToDataUrl = (blob: Blob): Promise<string> => {
      return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.onerror = () => reject(reader.error);
        reader.readAsDataURL(blob);
      });
    };

    // 检查剪贴板内容
    const checkClipboard = async () => {
      if (!navigator.clipboard || !navigator.clipboard.read) {
        console.warn('Clipboard API not supported');
        return;
      }

      try {
        if (!document.hasFocus()) {
          return; // 如果文档没有焦点，直接返回不尝试读取剪贴板
        }
        const clipboardItems = await navigator.clipboard.read();
        for (const item of clipboardItems) {
          if (item.types.includes('text/plain')) {
            const text = await item.getType('text/plain').then((blob) => blob.text());
            if (text !== lastSharedContent.value?.content && text !== lastSyncedContent.value?.content) {
              lastSyncedContent.value.content = text;
              lastSyncedContent.value.type = 'text';
              await syncClipboardContent(lastSyncedContent.value);
            }
          } else if (item.types.includes('image/png')) {
            const imageBlob = await item.getType('image/png');
            const base64Image = await blobToDataUrl(imageBlob);
            const cleanedBase64Image = base64Image.replace(/^data:image\/png;base64,/, '');
            if (cleanedBase64Image === lastSharedContent.value?.imageData) continue;
            if (cleanedBase64Image === lastSyncedContent.value?.imageData) continue;

            lastSyncedContent.value.content = `${Math.floor(Math.random() * 1e7)}.png`;
            lastSyncedContent.value.imageData = cleanedBase64Image;
            lastSyncedContent.value.type = 'image';
            await syncClipboardContent(lastSyncedContent.value);
          }
        }
      } catch (error) {
        console.error('Error reading clipboard:', error);
      }
    };

    // 获取共享剪贴板的最新记录
    let latestFetchInFlight = false;
    const fetchLastSharedContent = async () => {
      if (!isAuthenticated.value) return;
      if (latestFetchInFlight) return;
      latestFetchInFlight = true;

      try {
        const newId = newestItemId.value ?? 0;
        const response = await axios.get(`${API_URL}/api/clipboard/latest`, {
          params: { new: newId },
          headers: {
            Authorization: `Bearer ${token.value}`,
          },
        });

        const incomingItems: ClipboardItem[] = response.data || [];
        if (incomingItems.length > 0) {
          clipboardItems.value = mergeIncomingToTop(incomingItems, clipboardItems.value);
        }
        recomputeBounds(clipboardItems.value);
      } catch (error) {
        console.error('Error fetching latest shared content:', error);
        if (axios.isAxiosError(error) && error.response?.status === 401) {
          isAuthenticated.value = false;
          localStorage.removeItem('clipboard_token');
          localStorage.removeItem('clipboard_device');
          localStorage.removeItem('clipboard_token_expiry');
          stopPolling();
        }
      } finally {
        latestFetchInFlight = false;
      }
    };

    // 同步剪贴板内容到共享服务
    const syncClipboardContent = async (data: {
      type: 'text' | 'image';
      content: string;
      deviceInfo: string;
      imageData?: string;
    }) => {
      let dataToHash: string | Uint8Array;
      let sha256 = '';
      try {
      if (data.type === 'image' && data.imageData) {
        const imageDataBuffer = base64ToArrayBuffer(data.imageData);
        dataToHash = imageDataBuffer;
      } else if (data.type === 'text' && data.content) {
        dataToHash = normalizeTextForHash(data.content);
      } else {
        throw new Error('Invalid data');
      }

        sha256 = await calculateSHA256(dataToHash);
        const exists = await checkContentExists(sha256);

        if (exists) {
          showNotification('内容已存在', 'info');
          return;
        }
        const response = await axios.post(`${API_URL}/api/clipboard`, data, {
          headers: {
            Authorization: `Bearer ${token.value || localStorage.getItem('clipboard_token')}`,
          },
        });
        const createdItem = response.data as ClipboardItem;
        clipboardItems.value = mergeIncomingToTop([createdItem], clipboardItems.value);
        recomputeBounds(clipboardItems.value);
      } catch (error) {
        console.error('Error syncing clipboard content:', error);
        if (axios.isAxiosError(error) && error.response?.status === 401) {
          isAuthenticated.value = false;
          localStorage.removeItem('clipboard_token');
          localStorage.removeItem('clipboard_device');
          localStorage.removeItem('clipboard_token_expiry');
          stopPolling();
        }
      }
    };


    // 处理图片选择
    const handleImageSelect = (event: Event) => {
      const target = event.target as HTMLInputElement;
      if (target.files && target.files.length > 0) {
        selectedImage.value = target.files[0];
        createImagePreview();
      }
    };

    // 处理图片拖放
    const handleImageDrop = (event: DragEvent) => {
      if (event.dataTransfer && event.dataTransfer.files.length > 0) {
        const file = event.dataTransfer.files[0];
        if (file.type.startsWith('image/')) {
          selectedImage.value = file;
          createImagePreview();
        }
      }
    };

    // 创建图片预览
    const createImagePreview = () => {
      if (selectedImage.value) {
        const reader = new FileReader();
        reader.onload = (e) => {
          imagePreviewUrl.value = e.target?.result as string;
        };
        reader.readAsDataURL(selectedImage.value);
      }
    };

    // 清除图片选择
    const clearImageSelection = () => {
      selectedImage.value = null;
      imagePreviewUrl.value = '';
      // 同时重置文件输入
      const fileInput = document.getElementById('image-input') as HTMLInputElement;
      if (fileInput) {
        fileInput.value = '';
      }
    };

    // 复制到剪贴板
    const showCopyNotification = (message: string) => {
      const notification = document.createElement('div');
      notification.classList.add('copy-notification');
      notification.innerText = message;
      document.body.appendChild(notification);

      setTimeout(() => {
        document.body.removeChild(notification);
      }, 2000);
    };

    const fallbackCopyText = (text: string) => {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.setAttribute('readonly', '');
      textarea.style.position = 'fixed';
      textarea.style.top = '0';
      textarea.style.left = '0';
      textarea.style.opacity = '0';
      textarea.style.pointerEvents = 'none';
      document.body.appendChild(textarea);
      textarea.focus();
      textarea.select();
      textarea.setSelectionRange(0, textarea.value.length);

      let succeeded = false;
      try {
        succeeded = document.execCommand('copy');
      } catch (error) {
        succeeded = false;
      }
      document.body.removeChild(textarea);
      return succeeded;
    };

    const copyToClipboard = async (text: string) => {
      try {
        if (navigator.clipboard && window.isSecureContext) {
          try {
            await navigator.clipboard.writeText(text);
            showCopyNotification('已复制到剪贴板');
            return;
          } catch (error) {
            console.warn('Clipboard writeText failed, fallback to execCommand:', error);
          }
        }

        const fallbackSucceeded = fallbackCopyText(text);
        if (!fallbackSucceeded) {
          throw new Error('fallback copy failed');
        }
        showCopyNotification('已复制到剪贴板');
      } catch (error) {
        console.error('无法复制到剪贴板:', error);
        showNotification('复制失败，请手动复制', 'error');
      }
    };

    // 拆词 - 改进版，使用更智能的中文分词
    // 修改前端的splitAndShowWords函数
    const splitAndShowWords = async (text: string, index: number) => {
      try {
        if (wordSplitResults.value[index].length > 0) {
          wordSplitResults.value[index] = [];
          return;
        }
        const response = await axios.post(`${API_URL}/api/split-words`,
            {text},
            {
              headers: {
                'Authorization': `Bearer ${token.value}`
              }
            }
        );

        if (response.data && response.data.words) {
          wordSplitResults.value = {
            ...wordSplitResults.value,
            [index]: response.data.words
          };
        }
      } catch (error) {
        console.error('Error splitting words:', error);

        // 如果API调用失败，使用本地分词方法作为后备
        const pattern = /([a-zA-Z]+|[0-9]+|[\u4e00-\u9fa5]+|[\p{Punctuation}]+|[\p{Emoji}]+|[\p{Script=Hiragana}]+|[\p{Script=Katakana}]+|[\p{Script=Han}]+)/gu;
        const matches = text.match(pattern) || [];

        wordSplitResults.value = {
          ...wordSplitResults.value,
          [index]: matches
        };
      }
    };

    // Toggle word selection
    const toggleWordSelection = (word: string, index: number, wordIndex: number) => {
      if (!selectedWords.value[index]) {
        selectedWords.value[index] = [];
      }

      const existingIndex = selectedWords.value[index].findIndex(
          (selected) => selected.wordIndex === wordIndex
      );

      if (existingIndex !== -1) {
        // Word is already selected, remove it
        selectedWords.value[index].splice(existingIndex, 1);
      } else {
        // Add the word and sort by wordIndex
        selectedWords.value[index].push({word, wordIndex});
        selectedWords.value[index].sort((a, b) => a.wordIndex - b.wordIndex);
      }
    };

    // Copy merged words to clipboard
    const copyMergedWords = (index: number) => {
      const words = selectedWords.value[index] || [];
      const mergedText = words.map((item) => item.word).join(''); // Extract `word` and merge
      copyToClipboard(mergedText); // Use the existing `copyToClipboard` function
    };


    // 图片预览
    const previewImage = (item: ClipboardItem) => {
      if (item.imageData) {
        currentPreviewImage.value = item.imageData;
        imagePreviewModal.value = true;
      }
    };


    // 下载图片
    const downloadImage = (base64Data: UnwrapRef<ClipboardItem["imageData"]> | undefined) => {
      const link = document.createElement('a');
      link.href = `data:image/png;base64,${base64Data}`;
      link.download = `clipboard_image_${new Date().getTime()}.png`;
      link.click();
    };

    // 格式化时间 - 相对时间
    const formatTime = (timeStr: string) => {
      const date = new Date(timeStr);
      const now = new Date();
      const diffMs = now.getTime() - date.getTime();
      const diffSeconds = Math.floor(diffMs / 1000);
      const diffMinutes = Math.floor(diffSeconds / 60);
      const diffHours = Math.floor(diffMinutes / 60);
      const diffDays = Math.floor(diffHours / 24);

      if (diffSeconds < 60) {
        return '刚刚';
      } else if (diffMinutes < 60) {
        return `${diffMinutes}分钟前`;
      } else if (diffHours < 24) {
        return `${diffHours}小时前`;
      } else if (diffDays < 30) {
        return `${diffDays}天前`;
      } else {
        return `${date.getFullYear()}-${(date.getMonth() + 1).toString().padStart(2, '0')}-${date.getDate().toString().padStart(2, '0')}`;
      }
    };

    // 格式化完整时间
    const formatFullTime = (timeStr: string) => {
      const date = new Date(timeStr);
      return `${date.getFullYear()}-${(date.getMonth() + 1).toString().padStart(2, '0')}-${date.getDate().toString().padStart(2, '0')} ${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}:${date.getSeconds().toString().padStart(2, '0')}`;
    };


    const notification = ref({
      visible: false,
      message: '',
      type: 'success' as 'success' | 'error' | 'info', // success, error, info
      timeoutId: null as number | null,
    });

    const showNotification = (message: string, type: 'success' | 'error' | 'info' = 'info', duration = 3000) => {
      if (notification.value.timeoutId) {
        clearTimeout(notification.value.timeoutId);
      }
      notification.value.message = message;
      notification.value.type = type;
      notification.value.visible = true;
      notification.value.timeoutId = window.setTimeout(() => {
        notification.value.visible = false;
      }, duration);
    };

    // Convert Base64 string to ArrayBuffer
    const base64ToArrayBuffer = (base64: string): Uint8Array => {
      const binaryString = window.atob(base64);
      const len = binaryString.length;
      const bytes = new Uint8Array(len);
      for (let i = 0; i < len; i++) {
        bytes[i] = binaryString.charCodeAt(i);
      }
      return bytes;
    };


    // Calculate SHA256 hash
    const calculateSHA256 = async (input: string | Uint8Array): Promise<string> => {
      try {
        const data = typeof input === 'string' ? new TextEncoder().encode(input) : input;
        const hashBuffer = await crypto.subtle.digest('SHA-256', data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
        if (hashHex.length !== 64) {
          throw new Error("Invalid SHA256 hash length");
        }
        return hashHex;
      } catch (error) {
        console.error("SHA256 calculation failed:", error);
        showNotification("无法计算内容的校验和", "error");
        throw new Error("SHA256 calculation failed");
      }
    };


    // Check if content exists on the server
    const checkContentExists = async (sha256: string): Promise<boolean> => {
      try {
        const response = await axios.get(`${API_URL}/api/is_exist?sha256=${sha256}`, {
          headers: {
            Authorization: `Bearer ${token.value}`,
          },
        });
        return response.data.exists === true;
      } catch (error) {
        if (axios.isAxiosError(error) && error.response?.status === 404) {
          // If the endpoint itself doesn't exist, assume content doesn't exist to allow upload
          console.warn('/api/is_exist endpoint not found, proceeding with upload.');
          return false;
        }
        // Prefer allowing upload here; backend also does server-side dedup.
        console.warn('Error checking content existence, proceeding with upload:', error);
        return false;
      }
    };

    const handlePaste = async (event: ClipboardEvent) => {
      if (activeTab.value !== 'image' || !event.clipboardData) {
        return; // Only handle paste in image tab
      }

      const items = event.clipboardData.items;
      let imageFile: File | null = null;

      for (let i = 0; i < items.length; i++) {
        if (items[i].kind === 'file' && items[i].type.startsWith('image/')) {
          imageFile = items[i].getAsFile();
          break; // Take the first image found
        }
      }

      if (imageFile) {
        event.preventDefault(); // Prevent default paste behavior
        console.log("Pasted image:", imageFile.name);
        setSelectedImage(imageFile);
        showNotification("图片已粘贴", "success", 1500);
      }
    };

    const setSelectedImage = (file: File) => {
      selectedImage.value = file;
      createImagePreview();
    };

    const scrollToTop = () => {
      window.scrollTo({
        top: 0,
        behavior: 'smooth' // 平滑滚动
      });
    };

    return {
      // 身份认证相关
      isAuthenticated,
      isAuthenticating,
      authError,
      deviceInfo,
      password,
      authenticate,
      deviceIconClass,
      fetchClipboardItems,

      reload,

      // 剪贴板相关
      clipboardItems,
      displayItems,
      listLoading,
      listKey,
      isSearchMode,
      searchQuery,
      searchLoading,
      searchError,
      searchClipboardHistory,
      clearSearch,
      newClipboardContent,
      isUploading,
      uploadProgress,
      loading,
      addToClipboard,
      handleImageSelect,
      handleImageDrop,
      copyToClipboard,
      splitAndShowWords,
      downloadImage,
      formatTime,
      formatFullTime,
      wordSplitResults,
      selectedImage,
      imagePreviewUrl,
      activeTab,
      clearImageSelection,
      tryReadClipboard,
      autoRefresh,
      selectedWords,
      toggleWordSelection,
      copyMergedWords,
      oldestItemId,
      newestItemId,
      loadingMore,
      noMoreData,
      setupScrollListener,

      // 设备图标
      getDeviceIconClass,

      // 图片预览
      imagePreviewModal,
      currentPreviewImage,
      previewImage,

      //轮询
      startPolling,
      stopPolling,

      //通知
      notification,

      //image粘贴
      handlePaste,

      // 其他返回的属性...
      scrollToTop,
    };
  }
});
</script>

<style>
:root {
  --primary-color: #3498db;
  --primary-dark: #2980b9;
  --secondary-color: #2ecc71;
  --light-bg: #f5f7fa;
  --dark-bg: #34495e;
  --text-color: #2c3e50;
  --light-text: #7f8c8d;
  --border-color: #dfe6e9;
  --highlight-color: #e3f2fd;
  --error-color: #e74c3c;
  --success-color: #2ecc71;
  --warning-color: #f39c12;
  --shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  --border-radius: 15px;
}

* {
  box-sizing: border-box;
  outline: none;
  margin: 0;
  padding: 0;
}

body {
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  background-color: var(--light-bg);
  color: var(--text-color);
  line-height: 1.6;
  margin: 0;
  padding: 0;
  width: 100%;
}

.scroll-to-top-button {
  position: fixed;
  bottom: 20px;
  left: 20px;
  width: 40px; /* 按钮宽度 */
  height: 40px; /* 按钮高度 */
  background-color: var(--primary-color);
  color: white;
  border: none;
  border-radius: var(--border-radius);
  cursor: pointer;
  box-shadow: var(--shadow);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.3s ease;
}

.scroll-to-top-button:hover {
  background-color: var(--primary-dark);
}

/* 绘制箭头 */
.scroll-to-top-button::before,
.scroll-to-top-button::after {
  content: '';
  position: absolute;
  width: 12px; /* 箭头宽度 */
  height: 2px; /* 箭头厚度 */
  background-color: white;
  top: 50%;
  left: 50%;
  transform-origin: center;
}

/* 绘制左半边箭头 */
.scroll-to-top-button::before {
  transform: translate(-20%, -50%) rotate(45deg);
}

/* 绘制右半边箭头 */
.scroll-to-top-button::after {
  transform: translate(-80%, -50%) rotate(135deg);
}

/* 适配小屏幕设备 */
@media (max-width: 768px) {
  .scroll-to-top-button {
    width: 36px; /* 缩小按钮 */
    height: 36px;
  }

  .scroll-to-top-button::before,
  .scroll-to-top-button::after {
    width: 10px; /* 缩小箭头 */
  }
}



</style>
