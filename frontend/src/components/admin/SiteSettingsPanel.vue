<template>
  <div class="card overflow-hidden">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t("admin.settings.sites.title") }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.sites.description") }}
      </p>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-10">
      <div class="h-7 w-7 animate-spin rounded-full border-b-2 border-primary-600"></div>
    </div>

    <div v-else class="space-y-6 p-6">
      <!-- 提示：保存后全局生效，用户刷新后切换 -->
      <div class="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
        {{ t("admin.settings.sites.notice") }}
      </div>

      <!-- 全局开关 -->
      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t("admin.settings.sites.activeSite") }}
        </label>
        <div class="flex flex-wrap gap-3">
          <button
            v-for="site in siteOrder"
            :key="site"
            type="button"
            class="inline-flex min-h-10 items-center gap-2 rounded-lg border px-4 py-2 text-sm font-medium transition-colors"
            :class="activeSite === site
              ? 'border-primary-600 bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
              : 'border-gray-300 bg-white text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300'"
            @click="activeSite = site"
          >
            <span class="h-2 w-2 rounded-full" :class="activeSite === site ? 'bg-primary-600' : 'bg-gray-400'"></span>
            {{ siteLabel(site) }}
          </button>
        </div>
      </div>

      <!-- 两套站点资料 -->
      <div class="grid gap-6 lg:grid-cols-2">
        <section
          v-for="site in siteOrder"
          :key="site"
          class="rounded-xl border border-gray-200 p-5 dark:border-dark-700"
          :class="activeSite === site ? 'ring-1 ring-primary-500' : ''"
        >
          <div class="mb-4 flex items-center justify-between">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ siteLabel(site) }}</h3>
            <span v-if="activeSite === site" class="rounded-full bg-primary-100 px-2.5 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">
              {{ t("admin.settings.sites.current") }}
            </span>
          </div>

          <div class="space-y-4">
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.sites.name") }}</label>
              <input v-model="profiles[site].name" type="text" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </div>
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.sites.logo") }}</label>
              <input v-model="profiles[site].logo" type="text" placeholder="/logo.webp" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
            </div>
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.sites.subtitle") }}</label>
              <textarea v-model="profiles[site].subtitle" rows="2" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white"></textarea>
            </div>
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t("admin.settings.sites.footerHtml") }}</label>
              <textarea v-model="profiles[site].footer_html" rows="3" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 font-mono text-xs dark:border-dark-600 dark:bg-dark-800 dark:text-white"></textarea>
              <p class="mt-1 text-xs text-gray-400">{{ t("admin.settings.sites.footerHtmlHint") }}</p>
            </div>

            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800/50">
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t("admin.settings.sites.promo") }}</p>
              <div class="space-y-3">
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t("admin.settings.sites.promoBadge") }}</label>
                  <input v-model="profiles[site].promo.badge" type="text" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t("admin.settings.sites.promoTitle") }}</label>
                  <input v-model="profiles[site].promo.title" type="text" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t("admin.settings.sites.promoCta") }}</label>
                  <input v-model="profiles[site].promo.cta" type="text" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white" />
                </div>
                <div>
                  <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t("admin.settings.sites.promoItems") }}</label>
                  <textarea
                    :value="profiles[site].promo.items.join('\n')"
                    rows="3"
                    class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800 dark:text-white"
                    @input="setPromoItems(site, ($event.target as HTMLTextAreaElement).value)"
                  ></textarea>
                  <p class="mt-1 text-xs text-gray-400">{{ t("admin.settings.sites.promoItemsHint") }}</p>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>

      <!-- 保存按钮（独立于主设置表单） -->
      <div class="flex items-center justify-end gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
        <span v-if="saveError" class="mr-auto text-sm text-red-600 dark:text-red-400">{{ saveError }}</span>
        <span v-if="saved" class="mr-auto text-sm text-green-600 dark:text-green-400">{{ t("admin.settings.sites.saved") }}</span>
        <button
          type="button"
          class="inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-50"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? t("common.saving") : t("admin.settings.sites.save") }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { getSiteSettings, updateSiteSettings } from "@/api/admin/settings";
import { useAppStore } from "@/stores";
import type { SiteProfile } from "@/types";

const { t } = useI18n();
const appStore = useAppStore();

const siteOrder = ["site_a", "site_b"];
const activeSite = ref("site_a");
const profiles = ref<Record<string, SiteProfile>>({});
const loading = ref(true);
const saving = ref(false);
const saved = ref(false);
const saveError = ref("");

function emptyProfile(): SiteProfile {
  return { name: "", logo: "", subtitle: "", footer_html: "", promo: { badge: "", title: "", cta: "", items: [] } };
}

function siteLabel(site: string): string {
  return site === "site_a"
    ? t("admin.settings.sites.siteA")
    : t("admin.settings.sites.siteB");
}

function setPromoItems(site: string, value: string): void {
  profiles.value[site].promo.items = value.split("\n").map(s => s.trim()).filter(Boolean);
}

async function load() {
  loading.value = true;
  try {
    const data = await getSiteSettings();
    activeSite.value = data.active_site || "site_a";
    profiles.value = {
      site_a: { ...emptyProfile(), ...(data.site_profiles?.site_a || {}) },
      site_b: { ...emptyProfile(), ...(data.site_profiles?.site_b || {}) },
    };
  } catch {
    profiles.value = { site_a: emptyProfile(), site_b: emptyProfile() };
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  saved.value = false;
  saveError.value = "";
  try {
    const updated = await updateSiteSettings({
      active_site: activeSite.value,
      site_profiles: profiles.value,
    });
    activeSite.value = updated.active_site || activeSite.value;
    // 立即同步前台 store，管理员无需刷新即可在当前会话看到切换效果
    await appStore.fetchPublicSettings(true);
    saved.value = true;
  } catch (e) {
    saveError.value = e instanceof Error ? e.message : String(e);
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>
