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
    <div v-if="!isAuthenticated" class="auth-container">
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
      <button class="primary-button" @click="authenticate" :disabled="isAuthenticating">
        <span class="button-icon">🔐</span>
        {{ isAuthenticating ? '登录中...' : '登录' }}
      </button>
      <p v-if="authError" class="error">{{ authError }}</p>
    </div>

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
                :disabled="!newClipboardContent"
            >
              <span class="button-icon">➕</span> 添加到共享剪贴板
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
                <span>拖放图片到这里或点击选择</span>
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
                :disabled="!selectedImage"
            >
              <span class="button-icon">➕</span> 添加到共享剪贴板
            </button>
          </div>
        </div>
      </div>

      <div class="clipboard-history">
        <h2>剪贴板历史记录</h2>
        <div class="refresh-control">
          <span>自动刷新: </span>
          <label class="switch">
            <input type="checkbox" v-model="autoRefresh">
            <span class="slider round"></span>
          </label>
          <button class="refresh-button" @click="fetchClipboardItems()">
            <span class="refresh-icon">🔄</span>
          </button>
        </div>

        <div v-if="loading" class="loading-container">
          <div class="spinner"></div>
          <span>加载中...</span>
        </div>

        <div v-else-if="clipboardItems.length === 0" class="empty-state">
          <span class="empty-icon">📋</span>
          <p>暂无剪贴板记录</p>
          <p class="empty-hint">添加内容后将显示在这里</p>
        </div>

        <div v-else class="clipboard-list" :key="newestItemId?.toString() || 'defaultKey'">
        <div
              v-for="(item, index) in clipboardItems"
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

            <div v-if="index < 3 && item.type === 'image'" class="item-actions">
              <button class="action-button" @click="downloadImage(item.imageData)" title="下载图片">
                <span class="action-icon">💾</span> 下载
              </button>
              <button class="action-button" @click="previewImage(item)" title="预览图片">
                <span class="action-icon">🔍</span> 预览
              </button>
            </div>
          </div>
          <div v-if="loadingMore" class="load-more-spinner">
            <div class="spinner"></div>
            <span>加载更多...</span>
          </div>
          <div v-if="noMoreData && clipboardItems.length > 0" class="no-more-data">
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
  </div>
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
    const newClipboardContent = ref('');
    const selectedImage = ref<File | null>(null);
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

          // 设置滚动监听
          setupScrollListener();
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
      } else if (pollingInterval.value !== null) {
        clearInterval(pollingInterval.value);
        pollingInterval.value = null;
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

    // 启动轮询
    const startPolling = () => {
            if (autoRefresh.value && pollingInterval.value === null) {
              pollingInterval.value = window.setInterval(async () => {
                if (isAuthenticated.value) {
                  await fetchLastSharedContent(); // 定期更新共享剪贴板的最新记录
                  await checkClipboard();
                }
              }, 3000);

            }
        }
    ;

    // 停止轮询
    const stopPolling = () => {
      if (pollingInterval.value !== null) {
        clearInterval(pollingInterval.value);
        pollingInterval.value = null;
      }
    };

    const setupScrollListener = () => {
      const handleScroll = () => {
        if (noMoreData.value || loadingMore.value) return;

        const scrollPosition = window.innerHeight + window.pageYOffset;
        const documentHeight = document.documentElement.offsetHeight;

        // 当滚动到距离底部100px时，加载更多
        if (documentHeight - scrollPosition < 100) {
          fetchClipboardItems(true);
        }
      };

      window.addEventListener('scroll', handleScroll);

      // 组件卸载时移除监听
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
    });

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

        // 处理响应数据
        if (response.data && response.data.length > 0) {
          if (loadMore) {
            // 追加到已有数据的末尾
            clipboardItems.value = [...clipboardItems.value, ...response.data];
          } else {
            // 初始加载，直接覆盖
            clipboardItems.value = response.data;
          }

          // 更新最旧和最新的ID
          if (clipboardItems.value.length > 0) {
            const sortedItems = [...clipboardItems.value].sort((a, b) => a.id - b.id);
            oldestItemId.value = sortedItems[0].id;
            newestItemId.value = sortedItems[sortedItems.length - 1].id;
          }

          // 检查是否没有更多数据
          if (loadMore && response.data.length === 0) {
            noMoreData.value = true;
          }
        } else if (loadMore) {
          // 如果加载更多但没有数据返回
          noMoreData.value = true;
        }
      } catch (error) {
        console.error('Error fetching clipboard items:', error);

        // 如果 token 过期，处理认证状态
        if (axios.isAxiosError(error) && error.response?.status === 401) {
          isAuthenticated.value = false;
          localStorage.removeItem('clipboard_token');
          localStorage.removeItem('clipboard_device');
          localStorage.removeItem('clipboard_token_expiry');
        }
      } finally {
        if (loadMore) {
          loadingMore.value = false;
        } else {
          loading.value = false;
        }
      }
    };


    // 添加到剪贴板
    const addToClipboard = async () => {
      if (!isAuthenticated.value) return;

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

        await axios.post(`${API_URL}/api/clipboard`, payload, {
          headers: {
            'Authorization': `Bearer ${token.value}`
          }
        });

        // 重置表单
        if (activeTab.value === 'text') {
          newClipboardContent.value = '';
        } else {
          clearImageSelection();
        }

        // 重新获取剪贴板项目
        await fetchLastSharedContent();
      } catch (error) {
        console.error('Error adding to clipboard:', error);
      }
    };

    // 检查剪贴板内容
    const checkClipboard = async () => {
      if (!navigator.clipboard || !navigator.clipboard.read) {
        console.warn('Clipboard API not supported');
        return;
      }

      try {
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
            const reader = new FileReader();
            reader.onload = async () => {
              const base64Image = reader.result as string;
              const cleanedBase64Image = base64Image.replace(/^data:image\/png;base64,/, '');
              if (lastSharedContent.value && (cleanedBase64Image !== lastSharedContent.value.imageData) && lastSyncedContent.value && (cleanedBase64Image !== lastSyncedContent.value.imageData)) {
                lastSyncedContent.value.content = `${Math.floor(Math.random() * 1e7)}.png`;
                lastSyncedContent.value.imageData = cleanedBase64Image;
                lastSyncedContent.value.type = 'image';
                await syncClipboardContent(lastSyncedContent.value);
              }
            };
            reader.readAsDataURL(imageBlob);
          }
        }
      } catch (error) {
        console.error('Error reading clipboard:', error);
      }
    };

    // 获取共享剪贴板的最新记录
    const fetchLastSharedContent = async () => {
      if (!isAuthenticated.value || !newestItemId.value) return;

      try {
        const response = await axios.get(`${API_URL}/api/clipboard/latest?new=${newestItemId.value}`, {
          headers: {
            Authorization: `Bearer ${token.value}`,
          },
        });

        // 如果有新数据，添加到顶部
        if (response.data && response.data.length > 0) {
          // 添加新数据到顶部
          clipboardItems.value = [...response.data, ...clipboardItems.value];

          // 更新最新ID
          const sortedItems = [...clipboardItems.value].sort((a, b) => b.id - a.id);
          newestItemId.value = sortedItems[0].id;
          lastSharedContent.value = sortedItems[0];

          // 如果是首次加载，也更新最旧ID
          if (!oldestItemId.value && sortedItems.length > 0) {
            oldestItemId.value = sortedItems[sortedItems.length - 1].id;
          }
        }
        else{
          const sortedItems = [...clipboardItems.value].sort((a, b) => b.id - a.id);
          newestItemId.value = sortedItems[0].id;
          lastSharedContent.value = sortedItems[0];
        }
      } catch (error) {
        console.error('Error fetching latest shared content:', error);
      }
    };

    // 同步剪贴板内容到共享服务
    const syncClipboardContent = async (data: {
      type: 'text' | 'image';
      content: string;
      deviceInfo: string;
      imageData?: string;
    }) => {
      try {
        await axios.post(`${API_URL}/api/clipboard`, data, {
          headers: {
            Authorization: `Bearer ${localStorage.getItem('clipboard_token')}`,
          },
        });
        console.log('Clipboard content synced:', data);
      } catch (error) {
        console.error('Error syncing clipboard content:', error);
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
    const copyToClipboard = (text: string) => {
      navigator.clipboard.writeText(text)
          .then(() => {
            // 使用临时元素显示复制成功提示
            const notification = document.createElement('div');
            notification.classList.add('copy-notification');
            notification.innerText = '已复制到剪贴板';
            document.body.appendChild(notification);

            // 2秒后移除提示
            setTimeout(() => {
              document.body.removeChild(notification);
            }, 2000);
          })
          .catch(err => {
            console.error('无法复制到剪贴板:', err);
          });
    };

    // 拆词 - 改进版，使用更智能的中文分词
    // 修改前端的splitAndShowWords函数
    const splitAndShowWords = async (text: string, index: number) => {
      try {
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
        const pattern = /([a-zA-Z]+|[0-9]+|[\u4e00-\u9fa5]+|[\p{Punctuation}]|[\p{Emoji}]|[\p{Script=Hiragana}]|[\p{Script=Katakana}]|[\p{Script=Han}])/gu;
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

      // 剪贴板相关
      clipboardItems,
      newClipboardContent,
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
  --border-radius: 8px;
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

.container {
  max-width: 900px;
  margin: 0 auto;
  padding: 20px;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 15px;
  border-bottom: 1px solid var(--border-color);
}

h1 {
  color: var(--primary-color);
  font-size: 28px;
  font-weight: 600;
}

h2 {
  color: var(--dark-bg);
  font-size: 20px;
  margin-bottom: 15px;
  font-weight: 600;
}

.device-badge {
  display: flex;
  align-items: center;
  background-color: var(--highlight-color);
  padding: 7px 15px;
  border-radius: 10px;
  font-size: 14px;
  color: var(--primary-dark);
}

.device-icon {
  display: inline-block;
  width: 14px;
  height: 14px;
  margin-right: 8px;
  background-size: contain;
  background-repeat: no-repeat;
  background-position: center;
}

.device-badge .device-icon{
  display: inline-block;
  width: 20px;
  height: 20px;
  margin-right: 8px;
  background-size: contain;
  background-repeat: no-repeat;
  background-position: center;
}

.device-ios {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 384 512'%3E%3Cpath fill='%233498db' d='M318.7 268.7c-.2-36.7 16.4-64.4 50-84.8-18.8-26.9-47.2-41.7-84.7-44.6-35.5-2.8-74.3 20.7-88.5 20.7-15 0-49.4-19.7-76.4-19.7C63.3 141.2 4 184.8 4 273.5q0 39.3 14.4 81.2c12.8 36.7 59 126.7 107.2 125.2 25.2-.6 43-17.9 75.8-17.9 31.8 0 48.3 17.9 76.4 17.9 48.6-.7 90.4-82.5 102.6-119.3-65.2-30.7-61.7-90-61.7-91.9zm-56.6-164.2c27.3-32.4 24.8-61.9 24-72.5-24.1 1.4-52 16.4-67.9 34.9-17.5 19.8-27.8 44.3-25.6 71.9 26.1 2 49.9-11.4 69.5-34.3z'/%3E%3C/svg%3E");
}

.device-android {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 576 512'%3E%3Cpath fill='%232ecc71' d='M420.55,301.93a24,24,0,1,1,24-24,24,24,0,0,1-24,24m-265.1,0a24,24,0,1,1,24-24,24,24,0,0,1-24,24m273.7-144.48,47.94-83a10,10,0,1,0-17.27-10h0l-48.54,84.07a301.25,301.25,0,0,0-246.56,0L116.18,64.45a10,10,0,1,0-17.27,10h0l47.94,83C64.53,202.22,8.24,285.55,0,384H576c-8.24-98.45-64.54-181.78-146.85-226.55'/%3E%3C/svg%3E");
}

.device-windows {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 448 512'%3E%3Cpath fill='%233498db' d='M0 93.7l183.6-25.3v177.4H0V93.7zm0 324.6l183.6 25.3V268.4H0v149.9zm203.8 28L448 480V268.4H203.8v177.9zm0-380.6v180.1H448V32L203.8 65.7z'/%3E%3C/svg%3E");
}

.device-mac {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 448 512'%3E%3Cpath fill='%239b59b6' d='M400 32H48C21.5 32 0 53.5 0 80v352c0 26.5 21.5 48 48 48h352c26.5 0 48-21.5 48-48V80c0-26.5-21.5-48-48-48zM127 384.5c-5.5 9.6-17.8 12.8-27.3 7.3-9.6-5.5-12.8-17.8-7.3-27.3l14.3-24.7c16.1-4.9 29.3-1.1 39.6 11.4L127 384.5zm138.9-53.9H84c-11 0-20-9-20-20s9-20 20-20h51l65.4-113.2-20.5-35.4c-5.5-9.6-2.2-21.8 7.3-27.3 9.6-5.5 21.8-2.2 27.3 7.3l8.9 15.4 8.9-15.4c5.5-9.6 17.8-12.8 27.3-7.3 9.6 5.5 12.8 17.8 7.3 27.3l-85.8 148.6h62.1c20.2 0 31.5 23.7 22.7 40zm98.1 0h-29l19.6 33.9c5.5 9.6 2.2 21.8-7.3 27.3-9.6 5.5-21.8 2.2-27.3-7.3-32.9-56.9-57.5-99.7-74-128.1-16.7-29-4.8-58 7.1-67.8 13.1 22.7 32.7 56.7 58.9 102h52c11 0 20 9 20 20 0 11.1-9 20-20 20z'/%3E%3C/svg%3E");
}

.device-linux {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 448 512'%3E%3Cpath fill='%23f39c12' d='M220.8 123.3c1 .5 1.8 1.7 3 1.7 1.1 0 2.8-.4 2.9-1.5.2-1.4-1.9-2.3-3.2-2.9-1.7-.7-3.9-1-5.5-.1-.4.2-.8.7-.6 1.1.3 1.3 2.3 1.1 3.4 1.7zm-21.9 1.7c1.2 0 2-1.2 3-1.7 1.1-.6 3.1-.4 3.5-1.6.2-.4-.2-.9-.6-1.1-1.6-.9-3.8-.6-5.5.1-1.3.6-3.4 1.5-3.2 2.9.1 1 1.8 1.5 2.8 1.4zM420 403.8c-3.6-4-5.3-11.6-7.2-19.7-1.8-8.1-3.9-16.8-10.5-22.4-1.3-1.1-2.6-2.1-4-2.9-1.3-.8-2.7-1.5-4.1-2 9.2-27.3 5.6-54.5-3.7-79.1-11.4-30.1-31.3-56.4-46.5-74.4-17.1-21.5-33.7-41.9-33.4-72C311.1 85.4 315.7.1 234.8 0 132.4-.2 158 103.4 156.9 135.2c-1.7 23.4-6.4 41.8-22.5 64.7-18.9 22.5-45.5 58.8-58.1 96.7-6 17.9-8.8 36.1-6.2 53.3-6.5 5.8-11.4 14.7-16.6 20.2-4.2 4.3-10.3 5.9-17 8.3s-14 6-18.5 14.5c-2.1 3.9-2.8 8.1-2.8 12.4 0 3.9.6 7.9 1.2 11.8 1.2 8.1 2.5 15.7.8 20.8-5.2 14.4-5.9 24.4-2.2 31.7 3.8 7.3 11.4 10.5 20.1 12.3 17.3 3.6 40.8 2.7 59.3 12.5 19.8 10.4 39.9 14.1 55.9 10.4 11.6-2.6 21.1-9.6 25.9-20.2 12.5-.1 26.3-5.4 48.3-6.6 14.9-1.2 33.6 5.3 55.1 4.1.6 2.3 1.4 4.6 2.5 6.7v.1c8.3 16.7 23.8 24.3 40.3 23 16.6-1.3 34.1-11 48.3-27.9 13.6-16.4 36-23.2 50.9-32.2 7.4-4.5 13.4-10.1 13.9-18.3.4-8.2-4.4-17.3-15.5-29.7zM223.7 87.3c9.8-22.2 34.2-21.8 44-.4 6.5 14.2 3.6 30.9-4.3 40.4-1.6-.8-5.9-2.6-12.6-4.9 1.1-1.2 3.1-2.7 3.9-4.6 4.8-11.8-.2-27-9.1-27.3-7.3-.5-13.9 10.8-11.8 23-4.1-2-9.4-3.5-13-4.4-1-6.9-.3-14.6 2.9-21.8zM183 75.8c10.1 0 20.8 14.2 19.1 33.5-3.5 1-7.1 2.5-10.2 4.6 1.2-8.9-3.3-20.1-9.6-19.6-8.4.7-9.8 21.2-1.8 28.1 1 .8 1.9-.2-5.9 5.5-15.6-14.6-10.5-52.1 8.4-52.1zm-13.6 60.7c6.2-4.6 13.6-10 14.1-10.5 4.7-4.4 13.5-14.2 27.9-14.2 7.1 0 15.6 2.3 25.9 8.9 6.3 4.1 11.3 4.4 22.6 9.3 8.4 3.5 13.7 9.7 10.5 18.2-2.6 7.1-11 14.4-22.7 18.1-11.1 3.6-19.8 16-38.2 14.9-3.9-.2-7-1-9.6-2.1-8-3.5-12.2-10.4-20-15-8.6-4.8-13.2-10.4-14.7-15.3-1.4-4.9 0-9 4.2-12.3zm3.3 334c-2.7 35.1-43.9 34.4-75.3 18-29.9-15.8-68.6-6.5-76.5-21.9-2.4-4.7-2.4-12.7 2.6-26.4v-.2c2.4-7.6.6-16-.6-23.9-1.2-7.8-1.8-15 .9-20 3.5-6.7 8.5-9.1 14.8-11.3 10.3-3.7 11.8-3.4 19.6-9.9 5.5-5.7 9.5-12.9 14.3-18 5.1-5.5 10-8.1 17.7-6.9 8.1 1.2 15.1 6.8 21.9 16l19.6 35.6c9.5 19.9 43.1 48.4 41 68.9zm-1.4-25.9c-4.1-6.6-9.6-13.6-14.4-19.6 7.1 0 14.2-2.2 16.7-8.9 2.3-6.2 0-14.9-7.4-24.9-13.5-18.2-38.3-32.5-38.3-32.5-13.5-8.4-21.1-18.7-24.6-29.9s-3-23.3-.3-35.2c5.2-22.9 18.6-45.2 27.2-59.2 2.3-1.7.8 3.2-8.7 20.8-8.5 16.1-24.4 53.3-2.6 82.4.6-20.7 5.5-41.8 13.8-61.5 12-27.4 37.3-74.9 39.3-112.7 1.1.8 4.6 3.2 6.2 4.1 4.6 2.7 8.1 6.7 12.6 10.3 12.4 10 28.5 9.2 42.4 1.2 6.2-3.5 11.2-7.5 15.9-9 9.9-3.1 17.8-8.6 22.3-15 7.7 30.4 25.7 74.3 37.2 95.7 6.1 11.4 18.3 35.5 23.6 64.6 3.3-.1 7 .4 10.9 1.4 13.8-35.7-11.7-74.2-23.3-84.9-4.7-4.6-4.9-6.6-2.6-6.5 12.6 11.2 29.2 33.7 35.2 59 2.8 11.6 3.3 23.7.4 35.7 16.4 6.8 35.9 17.9 30.7 34.8-2.2-.1-3.2 0-4.2 0 3.2-10.1-3.9-17.6-22.8-26.1-19.6-8.6-36-8.6-38.3 12.5-12.1 4.2-18.3 14.7-21.4 27.3-2.8 11.2-3.6 24.7-4.4 39.9-.5 7.7-3.6 18-6.8 29-32.1 22.9-76.7 32.9-114.3 7.2zm257.4-11.5c-.9 16.8-41.2 19.9-63.2 46.5-13.2 15.7-29.4 24.4-43.6 25.5s-26.5-4.8-33.7-19.3c-4.7-11.1-2.4-23.1 1.1-36.3 3.7-14.2 9.2-28.8 9.9-40.6.8-15.2 1.7-28.5 4.2-38.7 2.6-10.3 6.6-17.2 13.7-21.1.3-.2.7-.3 1-.5.8 13.2 7.3 26.6 18.8 29.5 12.6 3.3 30.7-7.5 38.4-16.3 9-.3 15.7-.9 22.6 5.1 9.9 8.5 7.1 30.3 17.1 41.6 10.6 11.6 14 19.5 13.7 24.6zM173.3 148.7c2 1.9 4.7 4.5 8 7.1 6.6 5.2 15.8 10.6 27.3 10.6 11.6 0 22.5-5.9 31.8-10.8 4.9-2.6 10.9-7 14.8-10.4s5.9-6.3 3.1-6.6-2.6 2.6-6 5.1c-4.4 3.2-9.7 7.4-13.9 9.8-7.4 4.2-19.5 10.2-29.9 10.2s-18.7-4.8-24.9-9.7c-3.1-2.5-5.7-5-7.7-6.9-1.5-1.4-1.9-4.6-4.3-4.9-1.4-.1-1.8 3.7 1.7 6.5z'/%3E%3C/svg%3E");
}

.device-unknown {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 512 512'%3E%3Cpath fill='%237f8c8d' d='M256 8C119.043 8 8 119.083 8 256c0 136.997 111.043 248 248 248s248-111.003 248-248C504 119.083 392.957 8 256 8zm0 448c-110.532 0-200-89.431-200-200 0-110.495 89.472-200 200-200 110.491 0 200 89.471 200 200 0 110.53-89.431 200-200 200zm107.244-255.2c0 67.052-72.421 68.084-72.421 92.863V300c0 6.627-5.373 12-12 12h-45.647c-6.627 0-12-5.373-12-12v-8.659c0-35.745 27.1-50.034 47.579-61.516 17.561-9.845 28.324-16.541 28.324-29.579 0-17.246-21.999-28.693-39.784-28.693-23.189 0-33.894 10.977-48.942 29.969-4.057 5.12-11.46 6.071-16.666 2.124l-27.824-21.098c-5.107-3.872-6.251-11.066-2.644-16.363C184.846 131.491 214.94 112 261.794 112c49.071 0 101.45 38.304 101.45 88.8zM298 368c0 23.159-18.841 42-42 42s-42-18.841-42-42 18.841-42 42-42 42 18.841 42 42z'/%3E%3C/svg%3E");
}

/* 登录部分 */
.auth-container {
  background-color: #fff;
  padding: 30px;
  border-radius: var(--border-radius);
  box-shadow: var(--shadow);
  max-width: 450px;
  margin: 50px auto;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 500;
  color: var(--text-color);
}

.form-group input {
  width: 100%;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius);
  font-size: 16px;
  transition: border-color 0.3s, box-shadow 0.3s;
}

.form-group input:focus {
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(52, 152, 219, 0.2);
  outline: none;
}

.form-hint {
  display: block;
  font-size: 12px;
  color: var(--light-text);
  margin-top: 5px;
}

.error {
  color: var(--error-color);
  font-size: 14px;
  margin-top: 10px;
}

/* 按钮样式 */
.primary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background-color: var(--primary-color);
  color: white;
  border: none;
  padding: 12px 20px;
  border-radius: var(--border-radius);
  cursor: pointer;
  font-size: 16px;
  font-weight: 500;
  transition: background-color 0.2s, transform 0.1s;
  width: 100%;
}

.primary-button:hover {
  background-color: var(--primary-dark);
}

.primary-button:active {
  transform: translateY(1px);
}

.primary-button:disabled {
  background-color: var(--light-text);
  cursor: not-allowed;
}

.secondary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background-color: transparent;
  color: var(--primary-color);
  border: 1px solid var(--primary-color);
  padding: 11px 20px;
  border-radius: var(--border-radius);
  cursor: pointer;
  font-size: 16px;
  font-weight: 500;
  transition: background-color 0.2s, color 0.2s;
}

.secondary-button:hover {
  background-color: rgba(52, 152, 219, 0.1);
}

.button-icon {
  margin-right: 8px;
  font-size: 18px;
}

/* 主界面 */
.main-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.clipboard-input {
  background-color: white;
  padding: 25px;
  border-radius: var(--border-radius);
  box-shadow: var(--shadow);
}

.tabs {
  display: flex;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--border-color);
}

.tab-button {
  background: none;
  border: none;
  padding: 10px 20px;
  font-size: 16px;
  color: var(--light-text);
  cursor: pointer;
  position: relative;
  transition: color 0.3s;
  display: flex;
  align-items: center;
}

.tab-button.active {
  color: var(--primary-color);
}

.tab-button.active::after {
  content: '';
  position: absolute;
  bottom: -1px;
  left: 0;
  width: 100%;
  height: 3px;
  background-color: var(--primary-color);
}



.tab-icon {
  margin-right: 8px;
}

.tab-content {
  padding-top: 10px;
}

textarea {
  width: 100%;
  height: 120px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius);
  font-size: 16px;
  margin-bottom: 15px;
  resize: vertical;
  transition: border-color 0.3s;
}

textarea:focus {
  border-color: var(--primary-color);
  outline: none;
}

.image-preview-area {
  height: 180px;
  margin-bottom: 15px;
  position: relative;
}

.image-preview {
  width: 100%;
  height: 100%;
  position: relative;
  border-radius: var(--border-radius);
  overflow: hidden;
}

.image-preview img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background-color: #f8f9fa;
}

.image-dropzone {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  border: 2px dashed var(--border-color);
  border-radius: var(--border-radius);
  cursor: pointer;
  transition: border-color 0.3s, background-color 0.3s;
}

.image-dropzone:hover {
  border-color: var(--primary-color);
  background-color: rgba(52, 152, 219, 0.05);
}

.hidden-input {
  display: none;
}

.dropzone-label {
  display: flex;
  flex-direction: column;
  align-items: center;
  cursor: pointer;
  color: var(--light-text);
}

.upload-icon {
  font-size: 32px;
  margin-bottom: 10px;
}

.remove-image {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background-color: rgba(0, 0, 0, 0.5);
  color: white;
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.3s;
}

.remove-image:hover {
  background-color: rgba(0, 0, 0, 0.7);
}

.action-buttons {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.clipboard-history {
  background-color: white;
  padding: 25px;
  border-radius: var(--border-radius);
  box-shadow: var(--shadow);
}

.refresh-control {
  display: flex;
  align-items: center;
  margin-bottom: 15px;
  font-size: 14px;
  color: var(--light-text);
}

.switch {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 20px;
  margin: 0 10px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: #ccc;
  transition: .4s;
}

.slider:before {
  position: absolute;
  content: "";
  height: 16px;
  width: 16px;
  left: 2px;
  bottom: 2px;
  background-color: white;
  transition: .4s;
}

input:checked + .slider {
  background-color: var(--primary-color);
}

input:focus + .slider {
  box-shadow: 0 0 1px var(--primary-color);
}

input:checked + .slider:before {
  transform: translateX(20px);
}

.slider.round {
  border-radius: 20px;
}

.slider.round:before {
  border-radius: 50%;
}

.refresh-button {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--primary-color);
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  transition: background-color 0.3s;
}

.refresh-button:hover {
  background-color: rgba(52, 152, 219, 0.1);
}

.refresh-icon {
  font-size: 16px;
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
  color: var(--light-text);
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid rgba(52, 152, 219, 0.2);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: spin 1s infinite linear;
  margin-bottom: 10px;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
  color: var(--light-text);
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 10px;
  opacity: 0.5;
}

.empty-hint {
  font-size: 14px;
  margin-top: 5px;
  opacity: 0.7;
}

.clipboard-list {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.clipboard-item {
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius);
  padding: 20px;
  transition: transform 0.2s, box-shadow 0.2s;
}

.clipboard-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 5px 15px rgba(0, 0, 0, 0.08);
}

.clipboard-item.highlighter {
  border-color: var(--primary-color);
  background-color: var(--highlight-color);
}

.item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 7px;
  border-bottom: 1px solid var(--border-color);
}

.item-header .device-info {
  display: flex;
  align-items: center;
  font-weight: 500;
  font-size: 14px;
  color: var(--primary-color);
}

.timestamp {
  color: var(--light-text);
  font-size: 12px;
}

.item-content {
  margin-bottom: 15px;
  word-break: break-word;
}

.text-content {
  white-space: pre-wrap;
  line-height: 1.3;
  font-size: 15px;
  color: var(--text-color);
  max-height: 300px;
  overflow-y: auto;
}

.image-content {
  display: flex;
  justify-content: center;
}

.image-content img {
  max-width: 100%;
  max-height: 200px;
  border-radius: 4px;
  cursor: pointer;
  transition: opacity 0.3s;
}

.image-content img:hover {
  opacity: 0.9;
}

.item-actions {
  display: flex;
  flex-direction: row-reverse;
  flex-wrap: wrap;
  gap: 14px;
}

.load-more-spinner {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 0;
  color: var(--light-text);
}

.load-more-spinner .spinner {
  width: 30px;
  height: 30px;
  border: 2px solid rgba(52, 152, 219, 0.2);
  border-top-color: var(--primary-color);
  margin-bottom: 8px;
}

.no-more-data {
  text-align: center;
  padding: 20px 0;
  color: var(--light-text);
  font-size: 14px;
}

.action-button {
  display: inline-flex;
  align-items: center;
  background-color: #f8f9fa;
  color: var(--text-color);
  border: 2px solid var(--border-color);
  padding: 8px 12px;
  border-radius: 20px;
  font-size: 14px;
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;
}

.action-button:hover {
  background-color: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.action-icon {
  margin-right: 5px;
}

.split-words {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 15px;
  width: 100%;
  padding: 10px;
  background-color: #f8f9fa;
  border-radius: var(--border-radius);
}

.word-chip {
  display: inline-flex;
  background-color: white;
  border: 1px solid var(--primary-color);
  border-radius: 20px;
  padding: 4px 12px;
  font-size: 14px;
  cursor: pointer;
  transition: background-color 0.2s, color 0.2s;
  white-space: nowrap;
}

.word-chip:hover {
  background-color: var(--primary-color);
  color: white;
}

/* 图片预览模态框 */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(5px);
}

.modal-content {
  position: relative;
  max-width: 90%;
  max-height: 90%;
  background-color: white;
  border-radius: var(--border-radius);
  overflow: hidden;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
}

.modal-close {
  position: absolute;
  top: 15px;
  right: 15px;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background-color: rgba(0, 0, 0, 0.5);
  color: white;
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
  transition: background-color 0.3s;
}

.modal-close:hover {
  background-color: rgba(0, 0, 0, 0.7);
}

.preview-image {
  max-width: 100%;
  max-height: calc(90vh - 80px);
  object-fit: contain;
}

.modal-actions {
  padding: 15px;
  display: flex;
  justify-content: center;
  background-color: #f8f9fa;
}

.word-chip.selected {
  background-color: var(--primary-color);
  color: white;
  border-color: var(--primary-dark);
}

/* 复制提示 */
.copy-notification {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  background-color: var(--success-color);
  color: white;
  padding: 10px 20px;
  border-radius: 4px;
  font-size: 14px;
  box-shadow: 0 3px 8px rgba(0, 0, 0, 0.2);
  z-index: 1000;
  animation: fadeInOut 2s ease-in-out;
}

@keyframes fadeInOut {
  0% {
    opacity: 0;
    transform: translate(-50%, 20px);
  }
  20% {
    opacity: 1;
    transform: translate(-50%, 0);
  }
  80% {
    opacity: 1;
    transform: translate(-50%, 0);
  }
  100% {
    opacity: 0;
    transform: translate(-50%, -20px);
  }
}

/* 响应式设计 */
@media (max-width: 768px) {
  .container {
    padding: 15px;
  }

  .clipboard-item {
    padding: 15px;
  }

  .action-buttons {
    flex-direction: column;
  }

  .action-buttons button {
    width: 100%;
  }

  .action-button{
    padding: 4px 12px;
    border-radius: 15px;
    font-size: 13px;
  }


}

@media (max-width: 480px) {
  h1 {
    font-size: 24px;
  }

  h2 {
    font-size: 18px;
  }

  .container{
    padding: 0;
  }

  .app-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .device-badge {
    align-self: flex-start;
    padding: 4px 12px;
    border-radius: 15px;
    font-size: 12px;
    font-weight: 500;
    color: var(--primary-dark);
  }

  .device-badge .device-icon{
    width: 15px;
    height: 15px;
  }

  .tabs {
    flex-direction: row;
    gap: 10px;
    border-bottom: none;
  }

  .tab-button {
    width: 100%;
    border-bottom: 1px solid var(--border-color);
  }

  .tab-button.active::after {
    display: none;
  }

  .tab-button.active {
    background-color: var(--highlight-color);
  }

  .action-button{
    padding: 3px 10px;
    border-radius: 15px;
    font-size: 11px;
  }

}
</style>
