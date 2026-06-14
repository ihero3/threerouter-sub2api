(function() {
    // DOM 元素
    const slides = document.querySelectorAll('.slide');
    const navDots = document.querySelectorAll('.nav-dot');
    const prevBtn = document.getElementById('prevBtn');
    const nextBtn = document.getElementById('nextBtn');
    const progressBar = document.getElementById('progressBar');
    
    // 验证必要元素
    if (!slides.length || !navDots.length) {
        console.error('导航点或幻灯片元素未找到');
        return;
    }
    
    let currentIndex = 0;
    
    // 更新导航点高亮（根据当前滚动位置）
    function updateActiveDotFromScroll() {
        const scrollTop = window.scrollY;
        const windowHeight = window.innerHeight;
        
        // 找到当前最接近视口顶部的幻灯片
        let newIndex = 0;
        for (let i = 0; i < slides.length; i++) {
            const slideTop = slides[i].offsetTop;
            const slideBottom = slideTop + slides[i].offsetHeight;
            const viewportCenter = scrollTop + windowHeight / 2;
            
            if (viewportCenter >= slideTop && viewportCenter <= slideBottom) {
                newIndex = i;
                break;
            }
        }
        
        if (newIndex !== currentIndex) {
            currentIndex = newIndex;
            navDots.forEach((dot, i) => {
                dot.classList.toggle('active', i === currentIndex);
            });
        }
    }
    
    // 直接设置当前幻灯片索引（用于点击导航点时立即更新UI）
    function setCurrentIndex(index) {
        if (index < 0 || index >= slides.length) return;
        if (index === currentIndex) return;
        currentIndex = index;
        navDots.forEach((dot, i) => {
            dot.classList.toggle('active', i === currentIndex);
        });
    }
    
    // 更新进度条
    function updateProgressBar() {
        if (!progressBar) return;
        const scrollTop = window.scrollY;
        const docHeight = document.documentElement.scrollHeight - window.innerHeight;
        const progress = (scrollTop / docHeight) * 100;
        progressBar.style.width = progress + '%';
    }
    
    // 跳转到指定幻灯片
    function goToSlide(index) {
        if (index < 0 || index >= slides.length) return;
        if (index === currentIndex) return;
        
        // 立即更新导航点状态，提供即时反馈
        setCurrentIndex(index);
        
        // 滚动到目标幻灯片
        slides[index].scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
    
    // 下一页
    function nextSlide() {
        if (currentIndex < slides.length - 1) {
            goToSlide(currentIndex + 1);
        }
    }
    
    // 上一页
    function prevSlide() {
        if (currentIndex > 0) {
            goToSlide(currentIndex - 1);
        }
    }
    
    // 滚动事件处理（用于更新导航点高亮和进度条）
    let scrollTimeout = null;
    function onScroll() {
        if (scrollTimeout) clearTimeout(scrollTimeout);
        scrollTimeout = setTimeout(() => {
            updateActiveDotFromScroll();
            updateProgressBar();
        }, 50);
    }
    
    // 绑定导航点点击事件
    navDots.forEach((dot, index) => {
        dot.addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation();
            goToSlide(index);
        });
        // 移动端触摸支持
        dot.addEventListener('touchstart', (e) => {
            e.preventDefault();
            goToSlide(index);
        });
    });
    
    // 绑定按钮事件
    if (prevBtn) {
        prevBtn.addEventListener('click', (e) => {
            e.preventDefault();
            prevSlide();
        });
    }
    if (nextBtn) {
        nextBtn.addEventListener('click', (e) => {
            e.preventDefault();
            nextSlide();
        });
    }
    
    // 键盘事件
    document.addEventListener('keydown', (e) => {
        if (e.key === 'ArrowDown' || e.key === 'ArrowRight') {
            e.preventDefault();
            nextSlide();
        } else if (e.key === 'ArrowUp' || e.key === 'ArrowLeft') {
            e.preventDefault();
            prevSlide();
        } else if (e.key === 'Home') {
            e.preventDefault();
            goToSlide(0);
        } else if (e.key === 'End') {
            e.preventDefault();
            goToSlide(slides.length - 1);
        }
    });
    
    // 监听滚动
    window.addEventListener('scroll', onScroll);
    
    // 初始化
    setTimeout(() => {
        updateActiveDotFromScroll();
        updateProgressBar();
    }, 100);
    
    // 处理窗口大小变化
    window.addEventListener('resize', () => {
        setTimeout(updateActiveDotFromScroll, 100);
    });
})();
