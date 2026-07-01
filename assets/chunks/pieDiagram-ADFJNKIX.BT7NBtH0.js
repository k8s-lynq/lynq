import{_ as s,g as U,s as q,a as V,b as Z,q as j,p as H,l as w,c as J,E as K,I as Q,L as X,d as Y,y as ee,G as te}from"../app.fPKSMdmH.js";import{p as ae}from"./chunk-4BX2VUAB.BHhsSVnM.js";import{p as re}from"./treemap-KMMF4GRG.CUfV5gu9.js";import"./index.De4Wc8wj.js";import"./transform.D37oBwyH.js";import{d as G}from"./arc.Uj31lZxB.js";import{o as ie}from"./ordinal.BYWQX77i.js";import{d as se}from"./pie.d8arydmJ.js";import"./framework.aRVg8jgk.js";import"./select.BtBU3aSt.js";import"./index.C3pHsCe3.js";import"./step.CpIhdf-W.js";import"./theme.vfkaxMmt.js";import"./baseUniq.BrED52cr.js";import"./basePickBy.BqVzOape.js";import"./clone.BNZ3nTFu.js";import"./init.Gi6I4Gst.js";var oe=te.pie,D={sections:new Map,showData:!1},g=D.sections,C=D.showData,le=structuredClone(oe),ne=s(()=>structuredClone(le),"getConfig"),ce=s(()=>{g=new Map,C=D.showData,ee()},"clear"),pe=s(({label:e,value:a})=>{if(a<0)throw new Error(`"${e}" has invalid value: ${a}. Negative values are not allowed in pie charts. All slice values must be >= 0.`);g.has(e)||(g.set(e,a),w.debug(`added new section: ${e}, with value: ${a}`))},"addSection"),de=s(()=>g,"getSections"),ge=s(e=>{C=e},"setShowData"),me=s(()=>C,"getShowData"),W={getConfig:ne,clear:ce,setDiagramTitle:H,getDiagramTitle:j,setAccTitle:Z,getAccTitle:V,setAccDescription:q,getAccDescription:U,addSection:pe,getSections:de,setShowData:ge,getShowData:me},ue=s((e,a)=>{ae(e,a),a.setShowData(e.showData),e.sections.map(a.addSection)},"populateDb"),fe={parse:s(async e=>{const a=await re("pie",e);w.debug(a),ue(a,W)},"parse")},he=s(e=>`
  .pieCircle{
    stroke: ${e.pieStrokeColor};
    stroke-width : ${e.pieStrokeWidth};
    opacity : ${e.pieOpacity};
  }
  .pieOuterCircle{
    stroke: ${e.pieOuterStrokeColor};
    stroke-width: ${e.pieOuterStrokeWidth};
    fill: none;
  }
  .pieTitleText {
    text-anchor: middle;
    font-size: ${e.pieTitleTextSize};
    fill: ${e.pieTitleTextColor};
    font-family: ${e.fontFamily};
  }
  .slice {
    font-family: ${e.fontFamily};
    fill: ${e.pieSectionTextColor};
    font-size:${e.pieSectionTextSize};
    // fill: white;
  }
  .legend text {
    fill: ${e.pieLegendTextColor};
    font-family: ${e.fontFamily};
    font-size: ${e.pieLegendTextSize};
  }
`,"getStyles"),ve=he,Se=s(e=>{const a=[...e.values()].reduce((r,o)=>r+o,0),y=[...e.entries()].map(([r,o])=>({label:r,value:o})).filter(r=>r.value/a*100>=1).sort((r,o)=>o.value-r.value);return se().value(r=>r.value)(y)},"createPieArcs"),xe=s((e,a,y,$)=>{w.debug(`rendering pie chart
`+e);const r=$.db,o=J(),T=K(r.getConfig(),o.pie),A=40,l=18,p=4,c=450,m=c,u=Q(a),n=u.append("g");n.attr("transform","translate("+m/2+","+c/2+")");const{themeVariables:i}=o;let[b]=X(i.pieOuterStrokeWidth);b??(b=2);const E=T.textPosition,d=Math.min(m,c)/2-A,I=G().innerRadius(0).outerRadius(d),L=G().innerRadius(d*E).outerRadius(d*E);n.append("circle").attr("cx",0).attr("cy",0).attr("r",d+b/2).attr("class","pieOuterCircle");const f=r.getSections(),M=Se(f),O=[i.pie1,i.pie2,i.pie3,i.pie4,i.pie5,i.pie6,i.pie7,i.pie8,i.pie9,i.pie10,i.pie11,i.pie12];let h=0;f.forEach(t=>{h+=t});const _=M.filter(t=>(t.data.value/h*100).toFixed(0)!=="0"),v=ie(O);n.selectAll("mySlices").data(_).enter().append("path").attr("d",I).attr("fill",t=>v(t.data.label)).attr("class","pieCircle"),n.selectAll("mySlices").data(_).enter().append("text").text(t=>(t.data.value/h*100).toFixed(0)+"%").attr("transform",t=>"translate("+L.centroid(t)+")").style("text-anchor","middle").attr("class","slice"),n.append("text").text(r.getDiagramTitle()).attr("x",0).attr("y",-400/2).attr("class","pieTitleText");const k=[...f.entries()].map(([t,x])=>({label:t,value:x})),S=n.selectAll(".legend").data(k).enter().append("g").attr("class","legend").attr("transform",(t,x)=>{const F=l+p,R=F*k.length/2,N=12*l,B=x*F-R;return"translate("+N+","+B+")"});S.append("rect").attr("width",l).attr("height",l).style("fill",t=>v(t.label)).style("stroke",t=>v(t.label)),S.append("text").attr("x",l+p).attr("y",l-p).text(t=>r.getShowData()?`${t.label} [${t.value}]`:t.label);const P=Math.max(...S.selectAll("text").nodes().map(t=>(t==null?void 0:t.getBoundingClientRect().width)??0)),z=m+A+l+p+P;u.attr("viewBox",`0 0 ${z} ${c}`),Y(u,c,z,T.useMaxWidth)},"draw"),we={draw:xe},Oe={parser:fe,db:W,renderer:we,styles:ve};export{Oe as diagram};
