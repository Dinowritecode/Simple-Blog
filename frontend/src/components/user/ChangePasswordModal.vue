<script setup>
import { ref } from 'vue'
import { api } from '../../api'
import { useToast } from '../../composables/useToast'

const props = defineProps({ open: Boolean })
const emit = defineEmits(['close'])

const { toast } = useToast()
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const submitting = ref(false)
const error = ref('')

function close() {
  if (submitting.value) return
  oldPassword.value = newPassword.value = confirmPassword.value = ''
  error.value = ''
  emit('close')
}

async function submit() {
  if (!oldPassword.value || !newPassword.value) {
    error.value = '请填写原密码和新密码'
    return
  }
  if (newPassword.value.length < 6) {
    error.value = '新密码至少需要 6 位'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = '两次输入的新密码不一致'
    return
  }
  submitting.value = true
  error.value = ''
  try {
    await api.changePassword(oldPassword.value, newPassword.value)
    toast.success('密码修改成功，下次登录请使用新密码')
    close()
  } catch (e) {
    error.value = e.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <transition name="fade">
      <div v-if="open" class="modal-mask" @click.self="close">
        <div class="modal-card">
          <button class="modal-close" aria-label="关闭" @click="close">✕</button>
          <h3>修改密码</h3>
          <form @submit.prevent="submit">
            <label class="m-field">
              <span>原密码</span>
              <input v-model="oldPassword" type="password" autocomplete="current-password" placeholder="输入当前密码" />
            </label>
            <label class="m-field">
              <span>新密码（至少 6 位）</span>
              <input v-model="newPassword" type="password" autocomplete="new-password" placeholder="设置新密码" />
            </label>
            <label class="m-field">
              <span>确认新密码</span>
              <input v-model="confirmPassword" type="password" autocomplete="new-password" placeholder="再次输入新密码" />
            </label>
            <p v-if="error" class="form-error">{{ error }}</p>
            <div class="modal-actions">
              <button type="button" class="btn-sm ghost" @click="close">取消</button>
              <button type="submit" class="btn-sm" :disabled="submitting">
                {{ submitting ? '提交中…' : '确认修改' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<style scoped>
.form-error { color: #e5484d; font-size: 13.5px; margin: 2px 0 10px; }
.modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 18px; }
</style>
