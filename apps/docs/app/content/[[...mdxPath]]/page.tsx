import { generateStaticParamsFor, importPage } from "nextra/pages";
import { useMDXComponents as getMDXComponents } from "../../../mdx-components";
import type { Metadata } from 'next';
import { notFound } from "next/navigation";

export const generateStaticParams = generateStaticParamsFor("mdxPath");

function isValidMdxPath(mdxPath: string[] | undefined): boolean {
  if (!mdxPath || mdxPath.length === 0) return true; // root path
  // Reject Next.js internal paths (e.g. webpack hot-update requests)
  if (mdxPath[0] === "_next" || mdxPath[0] === "favicon.ico") return false;
  return true;
}

export async function generateMetadata(props: {
  params: Promise<{ mdxPath?: string[] }>
}): Promise<Metadata> {
  const params = await props.params;
  if (!isValidMdxPath(params.mdxPath)) notFound();
  const { metadata } = await importPage(params.mdxPath);
  return metadata;
}

const Wrapper = getMDXComponents().wrapper!;

export default async function Page(props: {
  params: Promise<{ mdxPath?: string[] }>
}) {
  const params = await props.params;
  if (!isValidMdxPath(params.mdxPath)) notFound();
  const {
    default: MDXContent,
    toc,
    metadata,
    sourceCode,
  } = await importPage(params.mdxPath);
  return (
    <Wrapper toc={toc} metadata={metadata} sourceCode={sourceCode}>
      <MDXContent {...props} params={params} />
    </Wrapper>
  );
}
