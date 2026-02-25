import DefaultTheme from 'vitepress/theme';
import Layout from './Layout.vue';
import HowItWorksDiagram from '../components/HowItWorksDiagram.vue';
import DependencyGraphVisualizer from '../components/DependencyGraphVisualizer.vue';
import TemplateBuilder from '../components/TemplateBuilder.vue';
import QuickstartStep from '../components/QuickstartStep.vue';
import CreationPolicyVisualizer from '../components/CreationPolicyVisualizer.vue';
import DeletionPolicyVisualizer from '../components/DeletionPolicyVisualizer.vue';
import ConflictPolicyVisualizer from '../components/ConflictPolicyVisualizer.vue';
import DependencyAnimationParallel from '../components/DependencyAnimationParallel.vue';
import DependencyAnimationCycle from '../components/DependencyAnimationCycle.vue';
import DependencyAnimationWaitForReady from '../components/DependencyAnimationWaitForReady.vue';
import RolloutAnimation from '../components/RolloutAnimation.vue';
import BlastRadiusAnimation from '../components/BlastRadiusAnimation.vue';
import LynqFlowDiagram from '../components/LynqFlowDiagram.vue';
import BlogPostMeta from '../components/BlogPostMeta.vue';
import BlogPostFooter from '../components/BlogPostFooter.vue';
import './custom.css';

export default {
  extends: DefaultTheme,
  Layout,
  enhanceApp({ app }) {
    // Register custom components globally
    app.component('HowItWorksDiagram', HowItWorksDiagram);
    app.component('DependencyGraphVisualizer', DependencyGraphVisualizer);
    app.component('TemplateBuilder', TemplateBuilder);
    app.component('QuickstartStep', QuickstartStep);
    app.component('CreationPolicyVisualizer', CreationPolicyVisualizer);
    app.component('DeletionPolicyVisualizer', DeletionPolicyVisualizer);
    app.component('ConflictPolicyVisualizer', ConflictPolicyVisualizer);
    app.component('DependencyAnimationParallel', DependencyAnimationParallel);
    app.component('DependencyAnimationCycle', DependencyAnimationCycle);
    app.component('DependencyAnimationWaitForReady', DependencyAnimationWaitForReady);
    app.component('RolloutAnimation', RolloutAnimation);
    app.component('BlastRadiusAnimation', BlastRadiusAnimation);
    app.component('LynqFlowDiagram', LynqFlowDiagram);
    app.component('BlogPostMeta', BlogPostMeta);
    app.component('BlogPostFooter', BlogPostFooter);
  }
};
